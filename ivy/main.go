package ivy

import (
	"fmt"
	"strings"

	"github.com/tcolgate/hugot"
	"github.com/tcolgate/hugot/bot"
	"github.com/tcolgate/hugot/handlers/command"
	ivy "robpike.io/ivy/mobile"
)

// New prometheus handler, returns a command and a webhook handler
func New() *command.Handler {
	return command.NewFunc(func(root *command.Command) error {
		root.Use = "ivy"
		root.Short = "evaluate an ivy APL expression"
		root.Long = "evaluates an expression using the ivy APL dialect. A large set of examples of ivy dialect can be seen here: https://github.com/robpike/ivy/blob/master/demo/demo.ivy"
		root.Example = "ivy +/(iota 6) o.== ?60000 rho 6"
		root.Run = ivyHandler
		return nil
	})
}

func ivyHandler(cmd *command.Command, w hugot.ResponseWriter, m *hugot.Message, args []string) error {
	out, err := ivy.Eval(strings.Join(args, " ") + "\n")
	if err != nil {
		return err
	}

	fmt.Fprint(w, out)
	return nil
}

// Register ivy handler against the default bot.
func Register() {
	bot.Command(New())
}
