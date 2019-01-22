// Copyright (c) 2019, The Decred developers
// See LICENSE for details.

package exchanges

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"sync"
	"time"
)

const (
	DefaultCurrency = "USD"
	// New data will be sought after the data expiry. Data will still be given to
	// a user until older than request expiry.
	DefaultDataExpiry    = "20m"
	DefaultRequestExpiry = "60m"
)

// ExchangeBotConfig is the configuration options for ExchangeBot.
// DataExpiry must be less than RequestExpiry.
// Recommend RequestExpiry > 2*DataExpiry, which will permit the exchange API
// request to fail a couple of times before the exchange's data is discarded.
type ExchangeBotConfig struct {
	Disabled      []string
	DataExpiry    string
	RequestExpiry string
	BtcIndex      string
	Indent        bool
}

// ExchangeBot monitors exchanges and processes updates. When an update is
// received from an exchange, the state is updated, and some convenient data
// structures are prepared. Make ExchangeBot with NewExchangeBot,
type ExchangeBot struct {
	*sync.RWMutex
	DcrBtcExchanges map[string]Exchange
	IndexExchanges  map[string]Exchange
	Exchanges       map[string]Exchange
	// BtcIndex is the (typically fiat) currency to which the DCR price should be
	// converted by default. Other conversions are available via a lookup in
	// indexMap, but with slightly lower performance.
	// 3-letter currency code, e.g. USD.
	BtcIndex     string
	indexMap     map[string]FiatIndices
	currentState ExchangeBotState
	// Both currentState and stateCopy hold the same information. currentState
	// is updated by ExchangeBot, and a copy stored in stateCopy. After creation,
	// stateCopy will never be updated, so can be used read-only by multiple
	// threads.
	stateCopy         *ExchangeBotState
	currentStateBytes []byte
	DataExpiry        time.Duration
	RequestExpiry     time.Duration
	minTick           time.Duration
	// udpateChans and quitChans hold update and exit channels requested by the
	// user.
	updateChans []chan *UpdateSignal
	quitChans   []chan struct{}
	// exchangeChan and indexChan are passed to the individual exchanges and
	// receive updates after a refresh is triggered.
	exchangeChan chan *ExchangeUpdate
	indexChan    chan *IndexUpdate
	client       *http.Client
	config       *ExchangeBotConfig
	// The failed flag is set when there are either no up-to-date Bitcoin-fiat
	// exchanges or no up-to-date Decred exchanges. IsFailed is a getter for failed.
	failed bool
}

// ExchangeBotState is the current known state of all exchanges, in a certain
// base currency, and a volume-averaged price and total volume in DCR.
type ExchangeBotState struct {
	BtcIndex    string                    `json:"btc_index"`
	Price       float64                   `json:"price"`
	Volume      float64                   `json:"volume"`
	DcrBtc      map[string]*ExchangeState `json:"dcr_btc_exchanges"`
	FiatIndices map[string]*ExchangeState `json:"btc_indices"`
}

// Copy an ExchangeState map.
func copyStates(m map[string]*ExchangeState) map[string]*ExchangeState {
	c := make(map[string]*ExchangeState)
	for k, v := range m {
		c[k] = v
	}
	return c
}

// Creates a pointer to a copy of the ExchangeBotState.
func (state ExchangeBotState) copy() *ExchangeBotState {
	state.DcrBtc = copyStates(state.DcrBtc)
	state.FiatIndices = copyStates(state.FiatIndices)
	return &state
}

// UpdateSignal is the update sent over the update channels, and includes an
// ExchangeBotState and a JSON-encoded byte array of the state.
// Token is the exchange which triggered the update.
type UpdateSignal struct {
	Token string
	State *ExchangeBotState
	Bytes []byte
}

// FiatIndices maps currency codes to Bitcoin exchange rates.
type FiatIndices map[string]float64

// IndexUpdate is sent from the Exchange to the ExchangeBot indexChan when new
// data is received.
type IndexUpdate struct {
	Token   string
	Indices FiatIndices
}

// BotChannels is passed to exchanges for communication with the Start loop.
type BotChannels struct {
	index    chan *IndexUpdate
	exchange chan *ExchangeUpdate
}

// UpdateChannels are requested by the user with ExchangeBot.UpdateChannels.
type UpdateChannels struct {
	Update chan *UpdateSignal
	Quit   chan struct{}
}

// NewExchangeBot constructs a new ExchangeBot with the provided configuration.
func NewExchangeBot(config *ExchangeBotConfig) (*ExchangeBot, error) {
	// Validate configuration
	if config.DataExpiry == "" {
		config.DataExpiry = DefaultDataExpiry
	}
	if config.RequestExpiry == "" {
		config.RequestExpiry = DefaultRequestExpiry
	}
	dataExpiry, err := time.ParseDuration(config.DataExpiry)
	if err != nil {
		return nil, fmt.Errorf("Unable to parse data expiration from %s", config.DataExpiry)
	}
	requestExpiry, err := time.ParseDuration(config.RequestExpiry)
	if err != nil {
		return nil, fmt.Errorf("Unable to parse request expiration from %s", config.RequestExpiry)
	}
	if requestExpiry < dataExpiry {
		return nil, fmt.Errorf("Request expiration must be longer than data expiration.")
	}
	if dataExpiry < time.Minute {
		return nil, fmt.Errorf("Expiration must be at least one minute.")
	}
	if config.BtcIndex == "" {
		config.BtcIndex = DefaultCurrency
	}
	if config.Disabled == nil {
		config.Disabled = []string{}
	}

	bot := &ExchangeBot{
		RWMutex:         new(sync.RWMutex),
		DcrBtcExchanges: make(map[string]Exchange),
		IndexExchanges:  make(map[string]Exchange),
		Exchanges:       make(map[string]Exchange),
		BtcIndex:        config.BtcIndex,
		indexMap:        make(map[string]FiatIndices),
		currentState: ExchangeBotState{
			BtcIndex:    config.BtcIndex,
			Price:       0,
			Volume:      0,
			DcrBtc:      make(map[string]*ExchangeState),
			FiatIndices: make(map[string]*ExchangeState),
		},
		currentStateBytes: []byte{},
		DataExpiry:        dataExpiry,
		RequestExpiry:     requestExpiry,
		minTick:           5 * time.Second,
		updateChans:       []chan *UpdateSignal{},
		quitChans:         []chan struct{}{},
		exchangeChan:      make(chan *ExchangeUpdate, 16),
		indexChan:         make(chan *IndexUpdate, 16),
		client:            new(http.Client),
		config:            config,
		failed:            false,
	}

	isDisabled := func(token string) bool {
		for _, tkn := range config.Disabled {
			if tkn == token {
				return true
			}
		}
		return false
	}

	channels := &BotChannels{
		index:    bot.indexChan,
		exchange: bot.exchangeChan,
	}

	buildExchange := func(token string, constructor func(*http.Client, *BotChannels) (Exchange, error), xcMap map[string]Exchange) {
		if isDisabled(token) {
			return
		}
		xc, err := constructor(bot.client, channels)
		if err != nil {
			return
		}
		xcMap[token] = xc
		bot.Exchanges[token] = xc
	}

	for token, constructor := range BtcIndices {
		buildExchange(token, constructor, bot.IndexExchanges)
	}

	for token, constructor := range DcrExchanges {
		buildExchange(token, constructor, bot.DcrBtcExchanges)
	}

	if len(bot.DcrBtcExchanges) == 0 || len(bot.IndexExchanges) == 0 {
		return nil, fmt.Errorf("Unable to create necessary exchanges.")
	}

	return bot, nil
}

// Start is the main ExchangeBot loop, reading from the exchange update channel
// and scheduling refresh cycles.
func (bot *ExchangeBot) Start(ctx context.Context, wg *sync.WaitGroup) {
	tick := time.NewTimer(time.Second)

	// Start refresh on all exchanges, and then change the updateTimes to
	// de-sync the updates.
	timeBetween := bot.DataExpiry / time.Duration(len(bot.Exchanges))
	idx := 0
	for _, xc := range bot.Exchanges {
		go func(xc Exchange, d int) {
			xc.Refresh()
			if !xc.IsFailed() {
				xc.Hurry(timeBetween * time.Duration(d))
			}
		}(xc, idx)
		idx++
	}

out:
	for {
		select {
		case update := <-bot.exchangeChan:
			bot.updateExchange(update)
			bot.signalUpdate(update.Token)
		case update := <-bot.indexChan:
			bot.updateIndices(update)
			bot.signalUpdate(update.Token)
		case <-tick.C:
			bot.Cycle()
		case <-ctx.Done():
			break out
		}
		tick = bot.nextTick()
	}
	if wg != nil {
		wg.Done()
	}
	bot.RLock()
	defer bot.RUnlock()
	for _, ch := range bot.quitChans {
		close(ch)
	}
}

// UpdateChannel returns a channel that will be sent updates.
func (bot *ExchangeBot) UpdateChannels() *UpdateChannels {
	update := make(chan *UpdateSignal, 16)
	quit := make(chan struct{})
	bot.Lock()
	defer bot.Unlock()
	bot.updateChans = append(bot.updateChans, update)
	bot.quitChans = append(bot.quitChans, quit)
	return &UpdateChannels{
		Update: update,
		Quit:   quit,
	}
}

// Send an update to any channels requested with bot.UpdateChannels().
func (bot *ExchangeBot) signalUpdate(token string) {
	var signal *UpdateSignal
	if bot.IsFailed() {
		signal = &UpdateSignal{
			Token: token,
			State: nil,
			Bytes: []byte{},
		}
	} else {
		signal = &UpdateSignal{
			Token: token,
			State: bot.State(),
			Bytes: bot.StateBytes(),
		}
	}
	for _, ch := range bot.updateChans {
		select {
		case ch <- signal:
		default:
		}
	}
}

// IsUpdated checks whether all enabled exchanges are up-to-date.
func (bot *ExchangeBot) IsUpdated() bool {
	oldestValid := time.Now().Add(-bot.RequestExpiry)
	for _, xc := range bot.Exchanges {
		lastUpdate := xc.LastUpdate()
		if lastUpdate.Before(oldestValid) {
			return false
		}
		if xc.LastFail().After(lastUpdate) {
			return false
		}
	}
	return true
}

// State is a copy of the current ExchangeBotState. A JSON-encoded byte array
// of the current state can be accessed through StateBytes().
func (bot *ExchangeBot) State() *ExchangeBotState {
	bot.RLock()
	defer bot.RUnlock()
	return bot.stateCopy
}

// ConvertedState returns an ExchangeBotState with a base of the provided
// currency code, if available.
func (bot *ExchangeBot) ConvertedState(code string) (*ExchangeBotState, error) {
	bot.RLock()
	defer bot.RUnlock()
	fiatIndices := make(map[string]*ExchangeState)
	for token, indices := range bot.indexMap {
		for symbol, price := range indices {
			if symbol == code {
				fiatIndices[token] = &ExchangeState{Price: price}
			}
		}
	}

	dcrPrice, volume := bot.processState(bot.currentState.DcrBtc, true)
	btcPrice, _ := bot.processState(fiatIndices, false)
	if dcrPrice == 0 || btcPrice == 0 {
		bot.failed = true
		return nil, fmt.Errorf("Unable to process price for currency %s.", code)
	}

	state := ExchangeBotState{
		BtcIndex:    code,
		Volume:      volume * btcPrice,
		Price:       dcrPrice * btcPrice,
		DcrBtc:      bot.currentState.DcrBtc,
		FiatIndices: fiatIndices,
	}

	return state.copy(), nil
}

// StateBytes is a JSON-encoded byte array of the currentState.
func (bot *ExchangeBot) StateBytes() []byte {
	bot.RLock()
	defer bot.RUnlock()
	return bot.currentStateBytes
}

// ConvertedStateBytes gives a JSON-encoded byte array of the currentState
// with a base of the provided currency code, if available.
func (bot *ExchangeBot) ConvertedStateBytes(symbol string) ([]byte, error) {
	state, err := bot.ConvertedState(symbol)
	if err != nil {
		return nil, err
	}
	var jsonBytes []byte
	if bot.config.Indent {
		jsonBytes, err = json.MarshalIndent(state, "", "    ")
	} else {
		jsonBytes, err = json.Marshal(state)
	}
	if err != nil {
		return nil, err
	}
	return jsonBytes, nil
}

// AvailableIndices creates a fresh slice of all available index currency codes.
func (bot *ExchangeBot) AvailableIndices() []string {
	bot.RLock()
	defer bot.RUnlock()
	var indices sort.StringSlice
	add := func(index string) {
		for _, symbol := range indices {
			if symbol == index {
				return
			}
		}
		indices = append(indices, index)
	}
	for _, fiatIndices := range bot.indexMap {
		for symbol, _ := range fiatIndices {
			add(symbol)
		}
	}
	sort.Sort(indices)
	return indices
}

// processState is a helper function to process a slice of ExchangeState into
// a price, and optionally a volume sum, and perform some cleanup along the way.
// If volumeAveraged is false, all exchanges are given equal weight in the avg.
func (bot *ExchangeBot) processState(states map[string]*ExchangeState, volumeAveraged bool) (float64, float64) {
	var priceAccumulator, volSum float64
	var deletions []string
	oldestValid := time.Now().Add(-bot.RequestExpiry)
	for token, state := range states {
		if bot.Exchanges[token].LastUpdate().Before(oldestValid) {
			deletions = append(deletions, token)
		}
		volume := 1.0
		if volumeAveraged {
			volume = state.Volume
		}
		volSum += volume
		priceAccumulator += volume * state.Price
	}
	for _, token := range deletions {
		delete(states, token)
	}
	if volSum == 0 {
		return 0, 0
	}
	return priceAccumulator / volSum, volSum
}

// updateExchange processes an update from a Decred-BTC Exchange.
func (bot *ExchangeBot) updateExchange(update *ExchangeUpdate) error {
	bot.Lock()
	defer bot.Unlock()
	bot.currentState.DcrBtc[update.Token] = update.State
	return bot.updateState()
}

// updateIndices processes an update from an Bitcoin index source, essentially
// a map pairing currency codes to bitcoin prices.
func (bot *ExchangeBot) updateIndices(update *IndexUpdate) error {
	bot.Lock()
	defer bot.Unlock()
	bot.indexMap[update.Token] = update.Indices
	price, hasCode := update.Indices[bot.config.BtcIndex]
	if hasCode {
		bot.currentState.FiatIndices[update.Token] = &ExchangeState{
			Price: price,
			Stamp: time.Now().Unix(),
		}
		return bot.updateState()
	}
	return nil
}

// Called from both updateIndices and updateExchange (under mutex lock).
func (bot *ExchangeBot) updateState() error {
	dcrPrice, volume := bot.processState(bot.currentState.DcrBtc, true)
	btcPrice, _ := bot.processState(bot.currentState.FiatIndices, false)
	if dcrPrice == 0 || btcPrice == 0 {
		bot.failed = true
		bot.stateCopy = nil
		return nil
	}

	bot.failed = false
	bot.currentState.Price = dcrPrice * btcPrice
	bot.currentState.Volume = volume
	var jsonBytes []byte
	var err error
	if bot.config.Indent {
		jsonBytes, err = json.MarshalIndent(bot.currentState, "", "    ")
	} else {
		jsonBytes, err = json.Marshal(bot.currentState)
	}
	if err != nil {
		return fmt.Errorf("Failed to write bytes")
	}
	bot.currentStateBytes = jsonBytes
	bot.stateCopy = bot.currentState.copy()
	return nil
}

// IsFailed is whether the failed flag was set during the last IndexUpdate
// or ExchangeUpdate. The failed flag is set when either no Bitcoin Index
// sources or no Decred Exchanges are up-to-date. Individual exchanges can
// be outdated/failed without IsFailed being false, as long as there is at least
// one Bitcoin index and one Decred exchange.
func (bot *ExchangeBot) IsFailed() bool {
	bot.RLock()
	defer bot.RUnlock()
	return bot.failed
}

// nextTick checks the exchanges' last update and fail times, and calculates
// when the next Cycle should run.
func (bot *ExchangeBot) nextTick() *time.Timer {
	tNow := time.Now()
	tOldest := tNow
	for _, xc := range bot.Exchanges {
		t := xc.LastTry()
		if t.Before(tOldest) {
			tOldest = t
		}
	}
	tSince := tNow.Sub(tOldest)
	tilNext := bot.DataExpiry - tSince
	if tilNext < bot.minTick {
		tilNext = bot.minTick
	}
	return time.NewTimer(tilNext)
}

// Cycle refreshes all expired exchanges.
func (bot *ExchangeBot) Cycle() {
	tNow := time.Now()
	for _, xc := range bot.Exchanges {
		if tNow.Sub(xc.LastTry()) > bot.DataExpiry {
			go xc.Refresh()
		}
	}
}

// Price gets the lastest Price in the default currency (BtcIndex).
func (bot *ExchangeBot) Price() float64 {
	bot.RLock()
	defer bot.RUnlock()
	return bot.currentState.Price
}
