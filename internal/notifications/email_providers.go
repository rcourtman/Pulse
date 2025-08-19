package notifications

// EmailProvider represents a pre-configured email provider template
type EmailProvider struct {
	Name         string `json:"name"`
	SMTPHost     string `json:"smtpHost"`
	SMTPPort     int    `json:"smtpPort"`
	TLS          bool   `json:"tls"`
	StartTLS     bool   `json:"startTLS"`
	AuthRequired bool   `json:"authRequired"`
	Instructions string `json:"instructions"`
}

// GetEmailProviders returns templates for popular email providers
func GetEmailProviders() []EmailProvider {
	return []EmailProvider{
		{
			Name:         "Gmail / Google Workspace",
			SMTPHost:     "smtp.gmail.com",
			SMTPPort:     587,
			TLS:          false,
			StartTLS:     true,
			AuthRequired: true,
			Instructions: `1. Enable 2-factor authentication on your Google account
2. Generate an App Password:
   - Go to https://myaccount.google.com/apppasswords
   - Select "Mail" and generate password
   - Use this password (not your regular password)
3. Use your full email as username`,
		},
		{
			Name:         "SendGrid",
			SMTPHost:     "smtp.sendgrid.net",
			SMTPPort:     587,
			TLS:          false,
			StartTLS:     true,
			AuthRequired: true,
			Instructions: `1. Sign up at https://sendgrid.com
2. Create an API Key:
   - Settings > API Keys > Create API Key
   - Choose "Restricted Access" with "Mail Send" permission
3. Username: "apikey" (literal string)
4. Password: Your API key
5. Verify sender identity in SendGrid dashboard`,
		},
		{
			Name:         "Mailgun",
			SMTPHost:     "smtp.mailgun.org",
			SMTPPort:     587,
			TLS:          false,
			StartTLS:     true,
			AuthRequired: true,
			Instructions: `1. Sign up at https://mailgun.com
2. Add and verify your domain
3. Get SMTP credentials:
   - Dashboard > Sending > Domain settings > SMTP credentials
4. Username: Default SMTP Login from dashboard
5. Password: Default Password from dashboard
Note: EU region uses smtp.eu.mailgun.org`,
		},
		{
			Name:         "Amazon SES (US East)",
			SMTPHost:     "email-smtp.us-east-1.amazonaws.com",
			SMTPPort:     587,
			TLS:          false,
			StartTLS:     true,
			AuthRequired: true,
			Instructions: `1. Set up Amazon SES in AWS Console
2. Verify your domain or email address
3. Create SMTP credentials:
   - SES Console > SMTP Settings > Create SMTP Credentials
4. Request production access (exit sandbox)
5. Use generated SMTP username and password
Other regions: us-west-2, eu-west-1, etc.`,
		},
		{
			Name:         "Microsoft 365 / Outlook",
			SMTPHost:     "smtp.office365.com",
			SMTPPort:     587,
			TLS:          false,
			StartTLS:     true,
			AuthRequired: true,
			Instructions: `IMPORTANT: Basic authentication (username/password) is being deprecated.
You MUST use an App Password:

1. Enable 2-factor authentication on your Microsoft account
2. Generate an App Password:
   - Go to https://account.microsoft.com/security
   - Click "Advanced security options"
   - Under "App passwords", create a new app password
   - Use this app password (not your regular password)
3. Username: Your full email address (e.g., user@domain.com)
4. Password: The app password you generated

Note: If using a work/school account, your admin may need to enable
"Authenticated SMTP" in the Microsoft 365 admin center.`,
		},
		{
			Name:         "Brevo (formerly Sendinblue)",
			SMTPHost:     "smtp-relay.brevo.com",
			SMTPPort:     587,
			TLS:          false,
			StartTLS:     true,
			AuthRequired: true,
			Instructions: `1. Sign up at https://brevo.com
2. Get SMTP credentials:
   - Account > SMTP & API > SMTP
3. Username: Your account email
4. Password: Your SMTP password (not login password)
5. Add sender in Senders management`,
		},
		{
			Name:         "Postmark",
			SMTPHost:     "smtp.postmarkapp.com",
			SMTPPort:     587,
			TLS:          false,
			StartTLS:     true,
			AuthRequired: true,
			Instructions: `1. Sign up at https://postmarkapp.com
2. Create a server and get credentials:
   - Servers > Your Server > API Tokens
3. Username: Your API token
4. Password: Your API token (same as username)
5. Verify sender signature`,
		},
		{
			Name:         "SparkPost",
			SMTPHost:     "smtp.sparkpostmail.com",
			SMTPPort:     587,
			TLS:          false,
			StartTLS:     true,
			AuthRequired: true,
			Instructions: `1. Sign up at https://sparkpost.com
2. Create API key with "Send via SMTP" permission
3. Username: SMTP_Injection
4. Password: Your API key
5. Verify sending domain
EU endpoint: smtp.eu.sparkpostmail.com`,
		},
		{
			Name:         "Resend",
			SMTPHost:     "smtp.resend.com",
			SMTPPort:     587,
			TLS:          false,
			StartTLS:     true,
			AuthRequired: true,
			Instructions: `1. Sign up at https://resend.com
2. Create an API key
3. Username: resend
4. Password: Your API key
5. Add and verify domain
Simple, developer-friendly service`,
		},
		{
			Name:         "SMTP2GO",
			SMTPHost:     "mail.smtp2go.com",
			SMTPPort:     587,
			TLS:          false,
			StartTLS:     true,
			AuthRequired: true,
			Instructions: `1. Sign up at https://smtp2go.com
2. Get SMTP credentials from dashboard
3. Username: Your SMTP username
4. Password: Your SMTP password
5. Add sender in Sender Domains`,
		},
		{
			Name:         "Custom SMTP Server",
			SMTPHost:     "",
			SMTPPort:     587,
			TLS:          false,
			StartTLS:     true,
			AuthRequired: true,
			Instructions: `Configure your own SMTP server:
- Common ports: 25 (no encryption), 587 (STARTTLS), 465 (TLS)
- Enable TLS/StartTLS for security
- Set AuthRequired=false for open relays (not recommended)
- Test connection before saving`,
		},
	}
}

// EmailProviderConfig contains enhanced email configuration
type EmailProviderConfig struct {
	EmailConfig
	Provider      string `json:"provider"`      // Provider name for quick setup
	ReplyTo       string `json:"replyTo"`       // Reply-to address
	MaxRetries    int    `json:"maxRetries"`    // Max send retries
	RetryDelay    int    `json:"retryDelay"`    // Seconds between retries
	RateLimit     int    `json:"rateLimit"`     // Max emails per minute
	StartTLS      bool   `json:"startTLS"`      // Use STARTTLS
	SkipTLSVerify bool   `json:"skipTLSVerify"` // Skip TLS cert verification
	AuthRequired  bool   `json:"authRequired"`  // Require authentication
}

// GetProviderDefaults returns default configuration for a provider
func GetProviderDefaults(providerName string) *EmailProviderConfig {
	providers := GetEmailProviders()
	for _, provider := range providers {
		if provider.Name == providerName {
			return &EmailProviderConfig{
				EmailConfig: EmailConfig{
					SMTPHost: provider.SMTPHost,
					SMTPPort: provider.SMTPPort,
					TLS:      provider.TLS,
				},
				Provider:   providerName,
				StartTLS:   provider.StartTLS,
				MaxRetries: 3,
				RetryDelay: 5,
				RateLimit:  60, // Default 60 emails/minute
			}
		}
	}
	return nil
}
