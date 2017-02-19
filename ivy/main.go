package ivy

import (
	"context"
	"fmt"
	"strings"

	"github.com/tcolgate/hugot"
	"github.com/tcolgate/hugot/bot"
	"github.com/tcolgate/hugot/handlers/command"
	ivy "robpike.io/ivy/mobile"
)

// New prometheus handler, returns a command and a webhook handler
func New() command.Commander {
	return command.New(
		"ivy",
		"the ivy APL-like calculator",
		ivyHandler)
}

func ivyHandler(ctx context.Context, w hugot.ResponseWriter, m *command.Message) error {
	m.Parse()

	out, err := ivy.Eval(strings.Join(m.Args(), " ") + "\n")
	if err != nil {
		return err
	}

	fmt.Fprint(w, out)
	return nil
}

func Register() {
	bot.Command(New())
}
