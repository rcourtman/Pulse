package notifications

import (
	"net/smtp"
	"net/textproto"
	"strconv"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSMTPLoginUsesExchangeOrderInsteadOfChallengeText(t *testing.T) {
	for _, prompts := range [][2]string{
		{"Username:", "Password:"},
		{"User Name", "Password"},
		{"", ""},
		{"Identifiant", "Mot de passe"},
		{"Password:", "Username:"},
	} {
		t.Run(prompts[0]+"/"+prompts[1], func(t *testing.T) {
			auth := LoginAuth("synthetic-user", "synthetic-password")
			mechanism, initial, err := auth.Start(&smtp.ServerInfo{Name: "localhost"})
			require.NoError(t, err)
			require.Equal(t, "LOGIN", mechanism)
			require.Nil(t, initial)
			username, err := auth.Next([]byte(prompts[0]), true)
			require.NoError(t, err)
			require.Equal(t, "synthetic-user", string(username))
			password, err := auth.Next([]byte(prompts[1]), true)
			require.NoError(t, err)
			require.Equal(t, "synthetic-password", string(password))
			response, err := auth.Next(nil, false)
			require.NoError(t, err)
			require.Nil(t, response)
		})
	}
}

func TestSMTPLoginRejectsIncompleteOrExcessChallenges(t *testing.T) {
	for _, challenges := range []int{0, 1, 3} {
		t.Run(strconv.Itoa(challenges), func(t *testing.T) {
			auth := LoginAuth("synthetic-user", "synthetic-password")
			_, _, err := auth.Start(&smtp.ServerInfo{Name: "localhost"})
			require.NoError(t, err)
			if challenges >= 1 {
				_, err = auth.Next([]byte("Username:"), true)
				require.NoError(t, err)
			}
			if challenges == 3 {
				_, err = auth.Next([]byte("Password:"), true)
				require.NoError(t, err)
				_, err = auth.Next([]byte("receiver-controlled-private-prompt"), true)
			} else {
				_, err = auth.Next(nil, false)
			}
			require.Error(t, err)
			require.Equal(t, NotificationFailureAuthentication, ClassifyNotificationFailureError(err))
			require.NotContains(t, err.Error(), "receiver-controlled-private-prompt")
		})
	}
}

func TestSMTPLoginStartResetsExchange(t *testing.T) {
	auth := LoginAuth("synthetic-user", "synthetic-password")
	for attempt := 0; attempt < 2; attempt++ {
		_, _, err := auth.Start(&smtp.ServerInfo{Name: "localhost"})
		require.NoError(t, err)
		response, err := auth.Next([]byte("Username:"), true)
		require.NoError(t, err)
		require.Equal(t, "synthetic-user", string(response))
		response, err = auth.Next([]byte("Password:"), true)
		require.NoError(t, err)
		require.Equal(t, "synthetic-password", string(response))
		_, err = auth.Next(nil, false)
		require.NoError(t, err)
	}
}

func TestSMTPAuthenticationPreservesGreetingFailure(t *testing.T) {
	client := newSMTPClientWithEHLO(t, []string{"421 authentication service temporarily unavailable"})
	defer client.Close()
	manager := NewEnhancedEmailManager(EmailProviderConfig{EmailConfig: EmailConfig{
		Username: "synthetic-user", Password: "synthetic-password",
	}})
	// Extension hides hello errors as a missing advertisement. Keep the
	// legacy fallback so Auth returns the original structured SMTP error.
	auth, err := manager.negotiateAuth(client)
	require.NoError(t, err)
	require.NotNil(t, auth)
	err = client.Auth(auth)
	var reply *textproto.Error
	require.ErrorAs(t, err, &reply)
	require.Equal(t, 421, reply.Code)
	require.Equal(t, NotificationFailureServerError, ClassifyNotificationFailureError(err))
}
