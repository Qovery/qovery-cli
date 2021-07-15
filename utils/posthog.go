package utils

import (
	"github.com/posthog/posthog-go"
	"time"
)

func Capture(command Name) {
	ph, err := posthog.NewWithConfig(
		"OxbbcR7J3ohTXEDGfsIL9KDlq5Gs080sbgfjrWYIOvU",
		posthog.Config{
			Endpoint: "https://ph.qovery.com",
		},
	)
	if err != nil {
		return
	}
	defer ph.Close()

	ctx, err := CurrentContext()
	if err != nil {
		return
	}

	err = ph.Enqueue(posthog.Capture{
		DistinctId: string(ctx.User),
		Event:      string(command),
		Timestamp:  time.Now(),
		Properties: ctx.ToPosthogProperties(),
	})
	if err != nil {
		return
	}
}
