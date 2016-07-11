package prometheus

import (
	"context"

	"github.com/tcolgate/hugot"
)

func (p *promH) explainCmd(ctx context.Context, w hugot.ResponseWriter, m *hugot.Message) error {
	m.Parse()

	return nil
}
