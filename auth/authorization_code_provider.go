package auth

import (
	"fmt"
	"strings"
)

// LocalCodeProvider holds the information needed to easily get an
// authorization code locally.
type LocalCodeProvider struct {
	Issuer
	oidcWellKnownEndpoints OIDCWellKnownEndpoints
	listener               AuthorizationCallbackListener
	osInteractor           OSInteractor
	state                  State
}

// AuthorizationCodeResult holds the needed code and redirect URI needed to exchange a
// authorization code for tokens
type AuthorizationCodeResult struct {
	Code        string
	RedirectURI string
}

// CallbackResponse holds the code gotten from the authorization callback.
// Error will hold an error struct if an error occurred.
type CallbackResponse struct {
	Code  string
	Error error
}

// AuthorizationCallbackListener abstracts listening for the authorization callback
type AuthorizationCallbackListener interface {
	GetCallbackURL() string
	AwaitResponse(response chan CallbackResponse, state string)
	Close()
}

// OSInteractor abstracts opening a url on the users OS
type OSInteractor interface {
	OpenURL(url string) error
}

// NewLocalCodeProvider allows for the easy setup of LocalCodeProvider
func NewLocalCodeProvider(
	issuer Issuer,
	oidcWellKnownEndpoints OIDCWellKnownEndpoints,
	callbackListener AuthorizationCallbackListener,
	osInteractor OSInteractor,
	state State) *LocalCodeProvider {
	return &LocalCodeProvider{
		issuer,
		oidcWellKnownEndpoints,
		callbackListener,
		osInteractor,
		state,
	}
}

// GetCode opens a URL to authenticate and authorize a user and then returns
// the authorization code that is sent to the callback. Additional scopes
// beyond openid and email can be sent by passing in arguments for
// <additionalScopes>.
func (cp *LocalCodeProvider) GetCode(challenge Challenge, additionalScopes ...string) (*AuthorizationCodeResult, error) {
	codeReceiverCh := make(chan CallbackResponse)
	defer close(codeReceiverCh)
	state := cp.state()
	go cp.listener.AwaitResponse(codeReceiverCh, state)

	if err := cp.osInteractor.OpenURL(fmt.Sprintf(
		"%s?audience=%s&scope=%s&response_type=code&client_id=%s&code_challenge=%s&code_challenge_method=%s&redirect_uri=%s&state=%s",
		cp.oidcWellKnownEndpoints.AuthorizationEndpoint,
		cp.Audience,
		strings.Join(append([]string{"openid", "email"}, additionalScopes...), " "),
		cp.ClientID,
		challenge.Code,
		challenge.Method,
		cp.listener.GetCallbackURL(),
		state,
	)); err != nil {
		return nil, err
	}

	callbackResult := <-codeReceiverCh

	if callbackResult.Error != nil {
		return nil, callbackResult.Error
	}

	cp.listener.Close()
	return &AuthorizationCodeResult{
		Code:        callbackResult.Code,
		RedirectURI: cp.listener.GetCallbackURL(),
	}, nil
}
