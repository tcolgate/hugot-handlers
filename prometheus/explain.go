package prometheus

import (
	"github.com/tcolgate/hugot"
	"golang.org/x/net/context"
)

func (p *promH) explainCmd(ctx context.Context, w hugot.ResponseWriter, m *hugot.Message) error {
	m.Parse()

	return nil
}
