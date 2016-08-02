package ivy

import (
	"context"
	"fmt"
	"strings"

	"github.com/tcolgate/hugot"
	ivy "robpike.io/ivy/mobile"
)

// New prometheus handler, returns a command and a webhook handler
func New() hugot.CommandHandler {
	return hugot.NewCommandHandler(
		"ivy",
		"the ivy APL-like calculator",
		ivyHandler,
		nil)
}

func ivyHandler(ctx context.Context, w hugot.ResponseWriter, m *hugot.Message) error {
	m.Parse()
	out, err := ivy.Eval(strings.Join(m.Args(), " ") + "\n")
	if err != nil {
		return err
	}

	fmt.Fprint(w, out)
	return nil
}
