package main

import (
	"fmt"
	"os"
	"os/signal"
	"runtime/pprof"
	"sync"

	"github.com/btcsuite/btclog"
	"github.com/dcrdata/dcrdata/dcrsqlite"
	"github.com/dcrdata/dcrdata/rpcutils"
	"github.com/decred/dcrrpcclient"
)

func init() {
	err := InitLogger()
	if err != nil {
		fmt.Printf("Unable to start logger: %v", err)
		os.Exit(1)
	}
}

func mainCore() int {
	// Parse the configuration file, and setup logger.
	cfg, err := loadConfig()
	if err != nil {
		fmt.Printf("Failed to load dcrdata config: %s\n", err.Error())
		return 1
	}

	if cfg.CPUProfile != "" {
		f, err := os.Create(cfg.CPUProfile)
		if err != nil {
			log.Fatal(err)
			return -1
		}
		pprof.StartCPUProfile(f)
		defer pprof.StopCPUProfile()
	}

	btclogger, err := btclog.NewLoggerFromWriter(log.Writer(), btclog.InfoLvl)
	if err != nil {
		log.Error("Unable to create logger for dcrrpcclient: ", err)
	}
	dcrrpcclient.UseLogger(btclogger)

	// Setup Sqlite db
	// dcrsqlite.UseLogger(btclogger)
	// db, err := dcrsqlite.InitDB(&dcrsqlite.DBInfo{cfg.DBFileName})
	// if err != nil {
	// 	log.Fatalf("InitDB failed: %v", err)
	// 	return 1
	// }

	// log.Infof("sqlite db successfully opened: %s", cfg.DBFileName)
	// defer db.Close()

	// Connect to node RPC server
	client, _, err := rpcutils.ConnectNodeRPC(cfg.DcrdServ, cfg.DcrdUser,
		cfg.DcrdPass, cfg.DcrdCert, cfg.DisableDaemonTLS)
	if err != nil {
		log.Fatalf("Unable to connect to RPC server: %v", err)
		return 1
	}

	infoResult, err := client.GetInfo()
	if err != nil {
		log.Errorf("GetInfo failed: %v", err)
		return 1
	}
	log.Info("Node connection count: ", infoResult.Connections)

	_, _, err = client.GetBestBlock()
	if err != nil {
		log.Error("GetBestBlock failed: ", err)
		return 2
	}

	// Sqlite output
	dcrsqlite.UseLogger(btclogger)
	dbInfo := dcrsqlite.DBInfo{FileName: cfg.DBFileName}
	//sqliteDB, err := dcrsqlite.InitDB(&dbInfo)
	sqliteDB, err := dcrsqlite.InitWiredDB(&dbInfo, client, activeChain)
	if err != nil {
		log.Errorf("Unable to initialize SQLite database: %v", err)
	}
	log.Infof("SQLite DB successfully opened: %s", cfg.DBFileName)
	defer sqliteDB.Close()

	// Ctrl-C to shut down.
	// Nothing should be sent the quit channel.  It should only be closed.
	quit := make(chan struct{})
	// Only accept a single CTRL+C
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt)

	// Start waiting for the interrupt signal
	go func() {
		<-c
		signal.Stop(c)
		// Close the channel so multiple goroutines can get the message
		log.Infof("CTRL+C hit.  Closing goroutines. Please wait.")
		close(quit)
		return
	}()

	// Resync db
	var waitSync sync.WaitGroup
	waitSync.Add(1)
	//go sqliteDB.SyncDB(&waitSync, quit)
	err = sqliteDB.SyncDBWithPoolValue(&waitSync, quit)
	if err != nil {
		log.Error(err)
	}

	waitSync.Wait()

	log.Print("Done!")

	return 0
}

func main() {
	os.Exit(mainCore())
}
