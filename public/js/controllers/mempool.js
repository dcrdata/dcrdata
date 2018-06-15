(() => {

    function incrementValue($el) {
        if ($el.size() > 0) {
            $el.text(
                parseInt($el.text()) + 1
            )
        }
    }

    function txTableRow(tx) {
        return `<tr class="flash">
            <td class="break-word"><span><a class="hash" href="/tx/${tx.hash}" title="${tx.hash}">${tx.hash}</a></span></td>
            <td class="mono fs15 text-right">${humanize.decimalParts(tx.total, false, 8, true)}</td>
            <td class="mono fs15 text-right">${tx.size} B</td>
            <td class="mono fs15 text-right" data-target="main.age" data-age="${tx.time}">${humanize.timeSince(tx.time)}</td>
        </tr>`
    }

    function voteTxTableRow(tx) {
        return `<tr>
            <td class="break-word"><span><a class="hash" title="(${tx.vote_info.mempool_ticket_index}) ${tx.vote_info.ticket_spent}" href="/tx/${tx.hash}">${tx.hash}</a></span></td>
            <td class="mono fs15">${tx.vote_info.vote_version}</td>
            <td class="mono fs15">${tx.vote_info.block_validation.validity}</td>
            <td class="mono fs15">${tx.vote_info.vote_choices[0].id}</td>
            <td class="mono fs15">${tx.vote_info.vote_choices[0].choice.Id}</td>
            <td class="mono fs15 text-right">${humanize.decimalParts(tx.total, false, 8, true)}</td>
            <td class="mono fs15">${tx.size} B</td>
            <td class="mono fs15 text-right" data-age="${tx.time}">${humanize.timeSince(tx.time)}</td>
        </tr>`
    }

    function txFlexTableRow(tx) {
        return `<div class="d-flex flex-table-row flash">
            <a class="hash truncate-hash" style="flex: 1 1 auto" href="/tx/${tx.hash}" title="${tx.hash}">${tx.hash}</a>
            <span style="flex: 0 0 60px" class="mono text-right ml-1">${tx.Type}</span>
            <span style="flex: 0 0 105px" class="mono text-right ml-1">${humanize.decimalParts(tx.total, false, 8, true)}</span>
            <span style="flex: 0 0 50px" class="mono text-right ml-1">${tx.size} B</span>
            <span style="flex: 0 0 65px" class="mono text-right ml-1" data-target="main.age" data-age="${tx.time}">${humanize.timeSince(tx.time)}</span>
        </div>`
    }

    function addTxRow(tx, target) {
        var rows = $(target).find('tr')
        var newRowHtml = $.parseHTML(txTableRow(tx))
        $(newRowHtml).insertBefore(rows.first())
    }

    app.register("homepageMempool", class extends Stimulus.Controller {
        static get targets() {
            return [
                "transactions",
                "numVote",
                "numTicket"
            ]
        }

        connect() {
            ws.registerEvtHandler("newtx", (evt) => {
                var txs = JSON.parse(evt)
                this.renderLatestTransactions(txs, true)
                keyNav(evt, false, true)
            })
            ws.registerEvtHandler("mempool", (evt) => {
                var m = JSON.parse(evt)
                this.renderLatestTransactions(m.latest, false)
                $(this.numTicketTarget).text(m.num_tickets)
                $(this.numVoteTarget).text(m.num_votes)
                keyNav(evt, false, true)
                ws.send("getmempooltxs", "")
            });
            ws.registerEvtHandler("getmempooltxsResp", (evt) => {
                var m = JSON.parse(evt)
                this.renderLatestTransactions(m.latest, true)
                keyNav(evt, false, true)
            })
        }

        disconnect() {
            ws.deregisterEvtHandlers("newtx")
            ws.deregisterEvtHandlers("mempool")
            ws.deregisterEvtHandlers("getmempooltxsResp")
        }

        renderLatestTransactions(txs, incremental) {
            _.each(txs, (tx, idx) => {
                if (incremental) {
                    incrementValue($(this["num" + tx.Type + "Target"]))
                }
                var rows = $(this.transactionsTarget).find(".flex-table-row")
                rows.last().remove()
                $($.parseHTML(txFlexTableRow(tx))).insertBefore(rows.first())
            })
        }
    })


    app.register("mempool", class extends Stimulus.Controller {
        static get targets() {
            return [
                "bestBlock",
                "bestBlockTime",
                "mempoolSize",
                "numVote",
                "numTicket",
                "numRevoke",
                "numRegular",
                "voteTransactions",
                "ticketTransactions",
                "revokeTransactions",
                "regularTransactions",
            ]
        }

        connect() {
            ws.registerEvtHandler("newtx", (evt) => {
                this.renderNewTxns(evt)
                keyNav(evt, false, true)
            })
            ws.registerEvtHandler("mempool", (evt) => {
                this.updateMempool(evt)
                ws.send("getmempooltxs", "")
            });
            ws.registerEvtHandler("getmempooltxsResp", (evt) => {
                this.handleTxsResp(evt)
                keyNav(evt, false, true)
            })
        }

        disconnect() {
            ws.deregisterEvtHandlers("newtx")
            ws.deregisterEvtHandlers("mempool")
            ws.deregisterEvtHandlers("getmempooltxsResp")
        }

        updateMempool(e) {
            var m = JSON.parse(e);
            $(this.numTicketTarget).text(m.num_tickets)
            $(this.numVoteTarget).text(m.num_votes)
            $(this.numRegularTarget).text(m.num_regular)
            $(this.numRevokeTarget).text(m.num_revokes)
            $(this.bestBlockTarget).text(m.block_height)
            $(this.bestBlockTimeTarget).attr('href', '/block/' + m.block_height)
            $(this.bestBlockTimeTarget).data('age', m.block_time)
            $(this.mempoolSizeTarget).text(m.formatted_size)
        }

        handleTxsResp(event) {
            var m = JSON.parse(event)

            var ticketsTable = _.map(m.tickets, txTableRow).join("")
            $(this.ticketTransactionsTarget).html($.parseHTML(ticketsTable))

            var transactionsTable = _.map(m.tx, txTableRow).join("")
            $(this.regularTransactionsTarget).html($.parseHTML(transactionsTable))

            // TODO handle no revokes scenario properly
            if (m.num_revokes > 0 && this.revokeTransactionsTarget) {
                var revokesTable = _.map(m.revokes, txTableRow).join("")
                $(this.revokeTransactionsTarget).html($.parseHTML(revokesTable))
            }

            var votesTable = _.map(m.votes, voteTxTableRow).join("")
            $(this.voteTransactionsTarget).html($.parseHTML(votesTable))
        }

        renderNewTxns(evt) {
            var txs = JSON.parse(evt)
            _.each(txs, (tx, idx) => {
                incrementValue($(this["num" + tx.Type + "Target"]))
                switch (tx.Type) {
                    case "Vote":
                        var rows = $(this[tx.Type.toLowerCase() + "TransactionsTarget"]).find('tr')
                        var newRow = voteTxTableRow(tx)
                        var newRowHtml = $.parseHTML(newRow)
                        $(newRowHtml).insertBefore(rows.first())
                        break
                    default:
                        addTxRow(tx, this[tx.Type.toLowerCase() + "TransactionsTarget"])
                }
            })
        }
    })

})()