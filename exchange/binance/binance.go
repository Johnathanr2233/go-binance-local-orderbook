package binance

import (
	"context"
	"log"
	"sync"

	"github.com/adshao/go-binance/v2"
	"github.com/jon4hz/go-binance-local-orderbook/config"
	"github.com/jon4hz/go-binance-local-orderbook/database"
	"github.com/jon4hz/go-binance-local-orderbook/exchange"
)

func InitWebsocket(config *config.Config) {
	//var response database.DatabaseInsert
	wsDepthHandler := func(event *binance.WsDepthEvent) {
		response := &database.BinanceDepthResponse{Response: event}
		exchange.BigU = event.FirstUpdateID
		exchange.SmallU = event.UpdateID
		// first time
		if exchange.Prev_u == 0 {
			// download snapshot
			if exchange.LastUpdateID == 0 {
				snap, err := downloadSnapshot(*config)
				if err != nil {
					log.Println("Error while downloading the snapshot")
					return
				}
				response = &database.BinanceDepthResponse{Snapshot: snap}
				err = response.InsertIntoDatabase(config.Database.DBTableMarketName)
				if err != nil {
					log.Println(err)
					// send notification
					return
				}
				log.Println("Inserted snapshot into db")
				exchange.LastUpdateID = snap.LastUpdateID
			}
			if exchange.SmallU >= exchange.LastUpdateID+1 && exchange.BigU <= exchange.LastUpdateID+1 {
				err := response.InsertIntoDatabase(config.Database.DBTableMarketName)
				if err != nil {
					log.Println(err)
					// send notification
					return
				}
				exchange.Prev_u = exchange.SmallU
				log.Println("Inserted first event successfully")
			}
			return

		} else if exchange.BigU >= exchange.Prev_u+1 {
			if exchange.BigU > exchange.Prev_u+1 {
				log.Printf("Warning, U = %d and prev_u = %d", exchange.BigU, exchange.Prev_u)
				// send notification
			}
			err := response.InsertIntoDatabase(config.Database.DBTableMarketName)
			if err != nil {
				log.Println(err)
				// send notification
			}
			exchange.Prev_u = exchange.SmallU
		} else {
			log.Println("Error")
		}

	}
	errHandler := func(err error) {
		log.Fatal(err)
	}
	var monitorWS func(sym string, ch chan struct{})
	monitorWS = func(sym string, ch chan struct{}) {
		go func() {
			<-ch
			// ws disconnected, try to re-establish.
			log.Printf("Websocket for %s crashed, spawning a new one.", sym)
			doneC, _, err := binance.WsDepthServe(sym, wsDepthHandler, errHandler)
			if err != nil {
				log.Printf("error registering symbol %s: %v", sym, err)
				return
			}
			monitorWS(sym, doneC)

			<-doneC
		}()
	}
	wg := sync.WaitGroup{}
	wg.Add(1)

	go func(sym string) {
		defer wg.Done()
		doneC, _, err := binance.WsDepthServe(sym, wsDepthHandler, errHandler)
		if err != nil {
			log.Printf("error registering symbol %s: %v", sym, err)
		}
		monitorWS(sym, doneC)

		<-doneC
	}(config.Exchange.Market)
	wg.Wait()

}

func downloadSnapshot(config config.Config) (res *binance.DepthResponse, err error) {
	client := binance.NewClient("", "")
	res, err = client.NewDepthService().Symbol(config.Exchange.Market).
		Limit(1000).
		Do(context.TODO())
	return

}
