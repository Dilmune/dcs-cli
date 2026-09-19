package ui

import "github.com/charmbracelet/lipgloss"

// Brand colors matching the Dilmune visual identity.
// Primary brand color is #d13e36 (warm red) from the logo/design system.
var (
	BrandPrimary = lipgloss.Color("#d13e36") // Main brand red
	BrandLight   = lipgloss.Color("#e8645d") // Lighter variant for accents
	BrandMuted   = lipgloss.Color("#c4a48a") // Warm muted for secondary text
	TextWhite    = lipgloss.Color("#e8e0d8") // Warm white (not pure white)
	TextMuted    = lipgloss.Color("#8a8278") // Warm gray
	TextDim      = lipgloss.Color("#5c564e") // Dim warm gray
	SuccessGreen = lipgloss.Color("#4ade80")
	ErrorRed     = lipgloss.Color("#ef4444")
	WarnYellow   = lipgloss.Color("#eab308")
	InfoBlue     = lipgloss.Color("#60a5fa")
	DarkBg       = lipgloss.Color("#1a1a2e")
)

// Gradient stops for the banner, from deep ember to bright flame.
var bannerGradient = []lipgloss.Color{
	lipgloss.Color("#8b1a15"),
	lipgloss.Color("#a52a22"),
	lipgloss.Color("#bf3a2e"),
	BrandPrimary,
	lipgloss.Color("#d9524a"),
	BrandLight,
	lipgloss.Color("#f08070"),
}

// Reusable styles.
var (
	Bold     = lipgloss.NewStyle().Bold(true)
	Title    = lipgloss.NewStyle().Bold(true).Foreground(BrandPrimary)
	Subtitle = lipgloss.NewStyle().Foreground(BrandLight)
	Muted    = lipgloss.NewStyle().Foreground(TextMuted)
	Dim      = lipgloss.NewStyle().Foreground(TextDim)
	Success  = lipgloss.NewStyle().Foreground(SuccessGreen)
	Error    = lipgloss.NewStyle().Foreground(ErrorRed)
	Warning  = lipgloss.NewStyle().Foreground(WarnYellow)
	Info     = lipgloss.NewStyle().Foreground(InfoBlue)
	Code     = lipgloss.NewStyle().Foreground(BrandLight).Bold(true)
	KeyStyle = lipgloss.NewStyle().Foreground(TextMuted).Width(14)
	ValStyle = lipgloss.NewStyle().Foreground(TextWhite)
)

// Resource statuses.
const (
	StatusActive       = "active"
	StatusRunning      = "running"
	StatusDeployed     = "deployed"
	StatusCompleted    = "completed"
	StatusSuccess      = "success"
	StatusProvisioning = "provisioning"
	StatusInstalling   = "installing"
	StatusDeploying    = "deploying"
	StatusBuilding     = "building"
	StatusPending      = "pending"
	StatusError        = "error"
	StatusFailed       = "failed"
	StatusSuspended    = "suspended"
	StatusDelinquent   = "delinquent"
	StatusDeleting     = "deleting"
	StatusDeleted      = "deleted"
)

// Billing statuses.
const (
	BillingGracePeriod = "grace_period"
	BillingSuspended   = StatusSuspended
	BillingDelinquent  = StatusDelinquent
)

// Event types used in log/event display.
const (
	EventError         = "error"
	EventSuccess       = "success"
	EventCompleted     = "completed"
	EventProgress      = "progress"
	EventDeploy        = "deploy"
	EventStatusChanged = "status_changed"
)

// StatusColor returns the appropriate color for a resource status.
func StatusColor(status string) lipgloss.Style {
	switch status {
	case StatusActive, StatusRunning, StatusDeployed, StatusCompleted, StatusSuccess:
		return Success
	case StatusProvisioning, StatusInstalling, StatusDeploying, StatusBuilding, StatusPending:
		return Warning
	case StatusError, StatusFailed, StatusSuspended, StatusDelinquent:
		return Error
	case StatusDeleting, StatusDeleted:
		return Muted
	default:
		return Muted
	}
}
