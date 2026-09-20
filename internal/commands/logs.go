package commands

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/signal"
	"time"

	"github.com/spf13/cobra"

	"github.com/dilmune/dcs-cli/internal/client"
	"github.com/dilmune/dcs-cli/internal/ui"
)

func newLogsCmd() *cobra.Command {
	var (
		serverFlag string
		siteFlag   string
		follow     bool
	)

	cmd := &cobra.Command{
		Use:   "logs",
		Short: "View server events and logs",
		Example: `  dcs logs --server web-1
  dcs logs --follow --server web-1
  dcs logs --follow --server web-1 --site my-site-id`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := requireAuth(); err != nil {
				return fmt.Errorf("auth: %w", err)
			}

			serverID, err := getServerID(serverFlag)
			if err != nil {
				return fmt.Errorf("getServerID: %w", err)
			}

			if follow {
				return followLogs(serverID, siteFlag)
			}

			return showRecentEvents(serverID)
		},
	}

	cmd.Flags().StringVar(&serverFlag, "server", "", "Server name or ID")
	cmd.Flags().StringVar(&siteFlag, "site", "", "Site ID to follow deployments for")
	cmd.Flags().BoolVarP(&follow, "follow", "f", false, "Follow logs in real-time")
	return cmd
}

func showRecentEvents(serverID string) error {
	resp, err := apiClient.Get(context.Background(), client.PathServerEvents(serverID), nil)
	if err != nil {
		return fmt.Errorf("fetch: %w", err)
	}

	if jsonOutput {
		ui.PrintJSONRaw(resp.Data)
		return nil
	}

	events, err := client.Decode[[]struct {
		Type      string `json:"type"`
		Message   string `json:"message"`
		CreatedAt string `json:"created_at"`
	}](resp)
	if err != nil {
		return fmt.Errorf("decode response: %w", err)
	}

	if len(events) == 0 {
		ui.PrintInfo("No events found.")
		return nil
	}

	fmt.Println()
	for _, e := range events {
		printEvent(e.Type, e.CreatedAt, e.Message)
	}
	fmt.Println()
	return nil
}

func followLogs(serverID, siteID string) error {
	ws, err := apiClient.DialWS()
	if err != nil {
		return fmt.Errorf("connect to live stream: %w", err)
	}
	defer ws.Close()

	topics := []string{
		client.WSTopicServer + serverID + client.WSTopicStatus,
		client.WSTopicServer + serverID + client.WSTopicEvents,
		client.WSTopicServer + serverID + client.WSTopicDatabases,
	}
	if siteID != "" {
		topics = append(topics, client.WSTopicSite+siteID+client.WSTopicStatus)
		topics = append(topics, client.WSTopicDeployment+siteID+client.WSTopicStatus)
	}

	for _, topic := range topics {
		if err := ws.Subscribe(topic); err != nil {
			return fmt.Errorf("subscribe to %s: %w", topic, err)
		}
	}

	fmt.Println()
	ui.PrintSuccess("Connected to live event stream. Press Ctrl+C to stop.")
	fmt.Println()

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt)

	msgCh := make(chan *client.WSMessage, 1)
	errCh := make(chan error, 1)

	go func() {
		for {
			msg, err := ws.ReadMessage()
			if err != nil {
				errCh <- err
				return
			}
			msgCh <- msg
		}
	}()

	pingTicker := time.NewTicker(30 * time.Second)
	defer pingTicker.Stop()

	for {
		select {
		case <-sig:
			fmt.Println()
			ui.PrintInfo("Disconnected.")
			return nil

		case err := <-errCh:
			return fmt.Errorf("connection lost: %w", err)

		case msg := <-msgCh:
			handleWSMessage(msg)

		case <-pingTicker.C:
			_ = ws.Ping()
		}
	}
}

func handleWSMessage(msg *client.WSMessage) {
	switch msg.Type {
	case client.WSTypeEvent:
		ts := time.Now().Format("15:04:05")

		var payload struct {
			ID        string `json:"id"`
			OldStatus string `json:"old_status"`
			NewStatus string `json:"new_status"`
			Message   string `json:"message"`
			Name      string `json:"name"`
		}
		if err := json.Unmarshal(msg.Payload, &payload); err != nil {
			return
		}

		var detail string
		switch {
		case payload.OldStatus != "" && payload.NewStatus != "":
			detail = fmt.Sprintf("[%s] %s → %s", msg.Topic, payload.OldStatus, payload.NewStatus)
		case payload.Message != "":
			detail = fmt.Sprintf("[%s] %s", msg.Topic, payload.Message)
		case payload.Name != "":
			detail = fmt.Sprintf("[%s] %s", msg.Topic, payload.Name)
		default:
			detail = fmt.Sprintf("[%s] %s", msg.Topic, msg.Event)
		}

		printEvent(msg.Event, ts, detail)
	case client.WSTypeError:
		ui.PrintError(fmt.Errorf("server: %s", string(msg.Payload)))
	}
}

func printEvent(eventType, timestamp, message string) {
	icon := ui.Muted.Render("●")
	switch eventType {
	case ui.EventError:
		icon = ui.Error.Render("●")
	case ui.EventSuccess, ui.EventCompleted:
		icon = ui.Success.Render("●")
	case ui.EventProgress, ui.EventDeploy, ui.EventStatusChanged:
		icon = ui.Info.Render("●")
	}
	fmt.Printf("  %s %s  %s\n", icon, timestamp, message)
}
