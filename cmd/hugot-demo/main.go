// Copyright (c) 2016 Tristan Colgate-McFarlane
//
// This file is part of hugot.
//
// hugot is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// hugot is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU General Public License for more details.
//
// You should have received a copy of the GNU General Public License
// along with hugot.  If not, see <http://www.gnu.org/licenses/>.

package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"

	"context"

	"github.com/golang/glog"
	bot "github.com/tcolgate/hugot"
	"github.com/tcolgate/hugot-handlers/prometheus"
	hslack "github.com/tcolgate/hugot/adapters/slack"

	"github.com/tcolgate/hugot"

	prom "github.com/prometheus/client_golang/api/prometheus"

	// Add some handlers
	"github.com/tcolgate/hugot/handlers/ping"
	"github.com/tcolgate/hugot/handlers/tableflip"
	"github.com/tcolgate/hugot/handlers/testcli"
	"github.com/tcolgate/hugot/handlers/testweb"
)

func bgHandler(ctx context.Context, w hugot.ResponseWriter) {
	fmt.Fprint(w, "Starting backgroud")
	<-ctx.Done()
	fmt.Fprint(w, "Stopping backgroud")
}

func httpHandler(w http.ResponseWriter, r *http.Request) {
	log.Printf("%#v", *r)
	w.Write([]byte("hello world"))
}

var slackToken = flag.String("token", os.Getenv("SLACK_TOKEN"), "Slack API Token")
var nick = flag.String("nick", "minion", "Bot nick")

func main() {
	flag.Parse()

	ctx := context.Background()
	//a, err := shell.New(*nick)
	//if err != nil {
	//		glog.Fatal(err)
	//	}

	a, err := hslack.New(*slackToken, *nick)
	if err != nil {
		glog.Fatal(err)
	}

	hugot.Handle(ping.New())
	hugot.Handle(testcli.New())
	hugot.Handle(tableflip.New())
	hugot.Handle(testweb.New())

	c, _ := prom.New(prom.Config{Address: "http://localhost:9090"})
	hugot.Handle(prometheus.New(&c, "http://localhost:9093", "alerts"))
	hugot.HandleBackground(hugot.NewBackgroundHandler("test bg", "testing bg", bgHandler))

	u, _ := url.Parse("http://localhost:8081")
	hugot.SetURL(u)

	go http.ListenAndServe(":8081", nil)

	bot.ListenAndServe(ctx, a, nil)
}
