package main

import (
	"log"
	"os"

	"github.com/slack-go/slack"
	"github.com/slack-go/slack/slackevents"
	"github.com/slack-go/slack/socketmode"
)

func main() {
	appToken := os.Getenv("SLACK_APP_TOKEN")
	botToken := os.Getenv("SLACK_BOT_TOKEN")
	debugEnv := os.Getenv("DEBUG")

	isDebug := false
	if debugEnv != "" {
		isDebug = true
	}

	client := slack.New(
		botToken,
		slack.OptionAppLevelToken(appToken),
		slack.OptionDebug(isDebug),
		slack.OptionLog(log.New(os.Stdout, "api: ", log.Lshortfile|log.LstdFlags)),
	)

	socketModeClient := socketmode.New(
		client,
		socketmode.OptionDebug(isDebug),
		socketmode.OptionLog(log.New(os.Stdout, "socketmode: ", log.Lshortfile|log.LstdFlags)),
	)

	go func() {
		for evt := range socketModeClient.Events {
			switch evt.Type {
			case socketmode.EventTypeEventsAPI:
				eventsAPIEvent, ok := evt.Data.(slackevents.EventsAPIEvent)
				if !ok {
					log.Printf("Ignored %+v\n", evt)
					continue
				}

				switch eventsAPIEvent.Type {
				case slackevents.CallbackEvent:
					innerEvent := eventsAPIEvent.InnerEvent
					switch ev := innerEvent.Data.(type) {
					case *slackevents.AppMentionEvent:
						_, _, err := client.PostMessage(ev.Channel, slack.MsgOptionText(ev.Text, false))
						if err != nil {
							log.Printf("failed posting message: %v", err)
							continue
						}
					}
				}
				socketModeClient.Ack(*evt.Request)
			}
		}
	}()

	socketModeClient.Run()
}
