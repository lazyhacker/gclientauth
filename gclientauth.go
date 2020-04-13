// Package gclientauth handles settings up a oAuth2 client and access token to
// talk to Google APIs from cli applications. The client libraries/APIs
// themselves don't handle authentication so it is necessary to first get the
// access token that is used to setup the API client to the APIs which this
// package handles.
//
// It wraps around golang.org/x/oauth2/google to provide some user
// friendly behavior and removes some boiler plate code for developers.
//
// The code is a mixture of various public Google tutorials and examples such
// as http://developers.google.com/youtube/v3/quickstart/go.
//
// In order to use this package:
//
//    1.  Create a new project on the Google API Console
//        (https://console.developers.google.com/).
//
//    2.  In the new project, enable the Google APIs to access.
//
//    3.  Setup up the credentials and download the client secret JSON
//        configuration from https://console.developers.google.com/apis/credentials
//
// TIP:
//
// If it is **desktop/other** credential is chosen then gclientauth will show
// the user an URL to visit in order toget a code that can be used to get an
// access token..
//
// If it is a **web application** then gclientauth will attempt to run a local
// webserver to get the code itself and create a token so the user don't have to
// do anything themselves.
//
//
// Example Usage:
//
// package main
//
//	   func main() {
//		  scopes := []string{youtube.YoutubeReadonlyScope}
//		  ctx := oauth2.NoContext
//		  token, config, err := gclientauth.GetGoogleOauth2Token(ctx, "client_secret.json", "accesstoken.json", scopes, false, "8080")
//		  ...
//		  cfg := config.Client(ctx, token)
//		  ...
//		  gp, err := googlephotos.New(cfg)
//		  ...
//		  res, err := gp.Albums.List().Do()
//		  for _, a := range res.Albums {
//			fmt.Printf("%v\n", a.Title)
//		  }
//	   }
package gclientauth // import "lazyhacker.dev/gclientauth"

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"runtime"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
)

// openURL opens a browser window to the specified location.
// This code originally appeared at:
//   http://stackoverflow.com/questions/10377243/how-can-i-launch-a-process-that-is-not-a-file-in-go
func openURL(url string) error {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "linux":
		cmd = exec.Command("xdg-open", url)
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", "http://localhost:4001/")
	case "darwin":
		cmd = exec.Command("open", url)
	default:
		return fmt.Errorf("Cannot open URL %s on this platform", url)
	}
	return cmd.Run()
}

// getCodeFromInstalled asks the user to input the code from the auth URL.
func getCodeFromInstalled(url string, browser bool) string {
	var code string
	var berr error
	if browser {
		berr = openURL(url)
	}

	if berr != nil || !browser {
		fmt.Printf("Visit the URL for the auth dialog: \n\t%v\n", url)
	}
	fmt.Print("Enter code: ")
	scanner := bufio.NewScanner(os.Stdin)
	for scanner.Scan() {
		code = scanner.Text()
		break
	}
	return code
}

// getCodeFromWeb returns a code that is used to exchange for a token.
func getCodeFromWeb(config *oauth2.Config, authURL, port string) string {
	hostname, err := url.Parse(config.RedirectURL)
	if err != nil {
		fmt.Errorf("Unable to determine the hostname from %v. %v", config.RedirectURL, err)
		return ""
	}
	codeCh, err := startWebServer(hostname.Hostname(), port)
	if err != nil {
		log.Printf("Unable to start a web server. %v", err)
		return ""
	}

	err = openURL(authURL)
	if err != nil {
		fmt.Errorf("Unable to open authorization URL in web server: %v", err)
	} else {
		fmt.Println("Your browser has been opened to an authorization URL.",
			" This program will resume once authorization has been provided.\n")
		fmt.Println(authURL)
	}

	// Wait for the web server to get the code.
	code := <-codeCh
	return code
}

// startWebServer starts a web server that waits for an oauth code in the
// three-legged auth flow.
func startWebServer(hostname, port string) (codeCh chan string, err error) {
	listener, err := net.Listen("tcp", fmt.Sprintf("%v:%v", hostname, port))
	if err != nil {
		log.Printf("Unable to do listener on %v. %v", hostname, err)
		return nil, err
	}
	codeCh = make(chan string)

	go http.Serve(listener, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		code := r.FormValue("code")
		codeCh <- code // send code to OAuth flow
		listener.Close()
		w.Header().Set("Content-Type", "text/plain")
		fmt.Fprintf(w, "Received code: %v\r\nYou can now safely close this browser window.", code)
	}))
	return codeCh, nil
}

func GetGoogleOauth2Token(ctx context.Context, credential, cachedtoken string, scopes []string, browser bool, port string) (*oauth2.Token, *oauth2.Config, error) {
	type cred struct {
	}

	var credtype struct {
		Web       *cred `json:"web"`
		Installed *cred `json:"installed"`
	}

	data, err := ioutil.ReadFile(credential)
	if err != nil {
		return nil, nil, fmt.Errorf("unable to read client credential file (%v). %v", credential, err)
	}

	config, err := google.ConfigFromJSON(data, scopes...)
	if err != nil {
		return nil, nil, fmt.Errorf("error parsing credential file. %v", err)
	}

	var token *oauth2.Token

	// Try to read the token from the cache file.
	// If an error occurs, do the three-legged OAuth flow because
	// the token is invalid or doesn't exist.
	t, err := ioutil.ReadFile(cachedtoken)
	if err == nil {
		err = json.Unmarshal(t, &token)
	}

	if (err != nil) || !token.Valid() {

		var code string
		// Redirect user to Google's consent page to ask for permission
		// for the scopes specified above.
		url := config.AuthCodeURL("state-token", oauth2.AccessTypeOffline)

		if err := json.Unmarshal(data, &credtype); err != nil {
			return nil, nil, fmt.Errorf("error parsing credential file. %v", err)
		}
		switch {
		case credtype.Installed != nil:
			code = getCodeFromInstalled(url, browser)
		case credtype.Web != nil:
			code = getCodeFromWeb(config, url, port)
		}
		// Exchanging for a token invalidates previous code so the same
		// code can't be used again.
		token, err = config.Exchange(ctx, code)
		if err != nil {
			return nil, nil, fmt.Errorf("unable to get valid token. code = \"%v\"\n%v", code, err)
		}
		data, err := json.Marshal(token)
		if err != nil {
			return nil, nil, fmt.Errorf("unable to encode the token for writing to cache. %v", err)
		}
		if err := ioutil.WriteFile(cachedtoken, data, 0644); err != nil {
			fmt.Errorf("(WARNING) Unable to write token to local cache.\n")
		}
	}
	return token, config, nil
}
