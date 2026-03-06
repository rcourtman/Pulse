package email

import (
	"bytes"
	"fmt"
	"html/template"
)

var magicLinkTemplate = template.Must(template.New("magic_link").Parse(`<!DOCTYPE html>
<html>
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>Sign in to Pulse</title>
</head>
<body style="font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif; margin: 0; padding: 0; background-color: #f5f5f5;">
<table role="presentation" style="width: 100%; border: 0; cellpadding: 0; cellspacing: 0;">
<tr><td style="padding: 40px 0; text-align: center;">
<table role="presentation" style="max-width: 480px; margin: 0 auto; background: #ffffff; border-radius: 8px; overflow: hidden; box-shadow: 0 1px 3px rgba(0,0,0,0.1);">
<tr><td style="padding: 32px 40px; text-align: center;">
<h1 style="margin: 0 0 16px; font-size: 24px; color: #1a1a1a;">Sign in to Pulse</h1>
<p style="margin: 0 0 24px; color: #666; font-size: 15px; line-height: 1.5;">
Click the button below to sign in to your Pulse dashboard. This link expires in 15 minutes.
</p>
<a href="{{.MagicLinkURL}}" style="display: inline-block; padding: 12px 32px; background: #2563eb; color: #ffffff; text-decoration: none; border-radius: 6px; font-size: 15px; font-weight: 500;">
Sign In
</a>
<p style="margin: 24px 0 0; color: #999; font-size: 13px; line-height: 1.5;">
If you didn't request this link, you can safely ignore this email.
</p>
</td></tr>
</table>
</td></tr>
</table>
</body>
</html>`))

// MagicLinkData holds template data for the magic link email.
type MagicLinkData struct {
	MagicLinkURL string
}

// RenderMagicLinkEmail renders the magic link HTML email.
func RenderMagicLinkEmail(data MagicLinkData) (html, text string, err error) {
	var buf bytes.Buffer
	if err := magicLinkTemplate.Execute(&buf, data); err != nil {
		return "", "", fmt.Errorf("render magic link template: %w", err)
	}

	textBody := fmt.Sprintf("Sign in to Pulse\n\nClick this link to sign in: %s\n\nThis link expires in 15 minutes. If you didn't request this, ignore this email.", data.MagicLinkURL)

	return buf.String(), textBody, nil
}

var trialVerificationTemplate = template.Must(template.New("trial_verification").Parse(`<!DOCTYPE html>
<html>
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>Verify your Pulse Pro trial request</title>
</head>
<body style="font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif; margin: 0; padding: 0; background-color: #f5f7f2;">
<table role="presentation" style="width: 100%; border: 0; cellpadding: 0; cellspacing: 0;">
<tr><td style="padding: 40px 0; text-align: center;">
<table role="presentation" style="max-width: 520px; margin: 0 auto; background: #ffffff; border-radius: 12px; overflow: hidden; box-shadow: 0 10px 30px rgba(20,38,31,0.08);">
<tr><td style="padding: 34px 40px; text-align: center;">
<h1 style="margin: 0 0 16px; font-size: 24px; color: #14261f;">Verify your Pulse Pro trial request</h1>
<p style="margin: 0 0 24px; color: #4d5f57; font-size: 15px; line-height: 1.6;">
Confirm this email address to continue setting up your 14-day Pulse Pro trial. This link expires in 20 minutes.
</p>
<a href="{{.VerifyURL}}" style="display: inline-block; padding: 12px 30px; background: #0f766e; color: #ffffff; text-decoration: none; border-radius: 8px; font-size: 15px; font-weight: 600;">
Verify Email
</a>
<p style="margin: 24px 0 0; color: #7a8a83; font-size: 13px; line-height: 1.5;">
If you did not request a Pulse Pro trial, you can safely ignore this email.
</p>
</td></tr>
</table>
</td></tr>
</table>
</body>
</html>`))

// TrialVerificationData holds template data for the trial verification email.
type TrialVerificationData struct {
	VerifyURL string
}

// RenderTrialVerificationEmail renders the trial verification HTML email.
func RenderTrialVerificationEmail(data TrialVerificationData) (html, text string, err error) {
	var buf bytes.Buffer
	if err := trialVerificationTemplate.Execute(&buf, data); err != nil {
		return "", "", fmt.Errorf("render trial verification template: %w", err)
	}

	textBody := fmt.Sprintf(
		"Verify your Pulse Pro trial request\n\nConfirm your email to continue setting up your 14-day Pulse Pro trial: %s\n\nThis link expires in 20 minutes. If you didn't request this, ignore this email.",
		data.VerifyURL,
	)

	return buf.String(), textBody, nil
}
