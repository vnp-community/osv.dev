// Package slack provides the Slack sender for the notification service.
package slack

import (
	"context"

	slackgo "github.com/slack-go/slack"
)

// Sender implements dispatch.SlackSender using the slack-go client.
type Sender struct {
	api *slackgo.Client
}

// New creates a Slack Sender with the given bot token.
func New(token string) *Sender {
	return &Sender{api: slackgo.New(token)}
}

// Send delivers a rich block-kit message to a Slack channel.
func (s *Sender) Send(ctx context.Context, channelID, title, description, viewURL string) error {
	blocks := []slackgo.Block{
		slackgo.NewHeaderBlock(
			slackgo.NewTextBlockObject("plain_text", "🔒 DefectDojo Alert", false, false),
		),
		slackgo.NewSectionBlock(
			slackgo.NewTextBlockObject("mrkdwn", "*"+title+"*", false, false),
			nil, nil,
		),
		slackgo.NewSectionBlock(
			slackgo.NewTextBlockObject("mrkdwn", description, false, false),
			nil, nil,
		),
		slackgo.NewActionBlock("",
			slackgo.NewButtonBlockElement("view_action", viewURL,
				slackgo.NewTextBlockObject("plain_text", "View in DefectDojo", false, false),
			),
		),
	}
	_, _, err := s.api.PostMessageContext(ctx, channelID, slackgo.MsgOptionBlocks(blocks...))
	return err
}
