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

	"context"

	"github.com/golang/glog"
	"github.com/tcolgate/hugot"
	"github.com/tcolgate/hugot-handlers/grafana"
	"github.com/tcolgate/hugot-handlers/ivy"
	"github.com/tcolgate/hugot-handlers/prometheus"
	"github.com/tcolgate/hugot-handlers/prometheus/am"
	hmm "github.com/tcolgate/hugot/adapters/mattermost"
	"github.com/tcolgate/hugot/bot"

	prom "github.com/prometheus/client_golang/api"

	// Add some handlers
	"github.com/tcolgate/hugot/handlers/command/ping"
	"github.com/tcolgate/hugot/handlers/command/testcli"
	"github.com/tcolgate/hugot/handlers/hears/tableflip"
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

var eurl = flag.String("url", "http://localhost", "url")
var port = flag.String("port", "8090", "url")
var team = flag.String("team", "test", "team name")
var mail = flag.String("email", "hugot@test.net", "Bot mail")
var pass = flag.String("pass", "hugot", "Bot pass")

func main() {
	flag.Parse()

	ctx := context.Background()
	a, err := hmm.New("http://localhost:8065", *team, *mail, *pass)

	if err != nil {
		glog.Fatal(err)
	}

	ping.Register()
	testcli.Register()
	ivy.Register()
	tableflip.Register()
	testweb.Register()

	grafana.Register(
		http.DefaultClient,
		"http://localhost:3000",
		"eyJrIjoiVHVKNVBmY0Z1VmFEdDRZSW9Wc2ZmSENyckV3bTJ5MDMiLCJuIjoicHJvbSIsImlkIjoxfQ==")

	c, _ := prom.NewClient(prom.Config{Address: "http://localhost:9090"})
	amc, _ := am.New(am.Config{Address: "http://localhost:9093"})
	prometheus.Register(&c, amc, nil)

	u, _ := url.Parse("http://localhost:8090")
	bot.SetURL(u)

	glog.Info(bot.URL())

	go http.ListenAndServe(":8090", nil)

	bot.ListenAndServe(ctx, nil, a)
}
