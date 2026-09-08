// reality-target-audit prints fresh observations for manual candidate review.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"log"
	"os"
	"os/signal"

	"github.com/kazeyukiro/3m-ui/backend/internal/mihomo/realityscan"
)

func main() {
	vantage := flag.String("vantage", "", "required: describe the server/network used for this audit")
	flag.Parse()
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	report, err := realityscan.Audit(ctx, *vantage)
	if err != nil {
		log.Fatal(err)
	}
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(report); err != nil {
		log.Fatal(err)
	}
}
