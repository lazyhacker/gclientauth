Package gclientauth handles settings up a oAuth2 client and access token to
talk to Google APIs from cli applications. The client libraries/APIs
themselves don't handle authentication so it is necessary to first get the
access token that is used to setup the API client to the APIs which this
package handles.

It wraps around golang.org/x/oauth2/google to provide some user
friendly behavior and removes some boiler plate code for developers.

The code is a mixture of various public Google tutorials and examples such
as http://developers.google.com/youtube/v3/quickstart/go.

In order to use this package:

   1.  Create a new project on the Google API Console
       (https://console.developers.google.com/).
   1.  In the new project, enable the Google APIs to access.
   1.  Setup up the credentials and download the client secret JSON
       configuration from https://console.developers.google.com/apis/credentials

TIP:

If it is **desktop/other** credential is chosen then gclientauth will show
the user an URL to visit in order toget a code that can be used to get an
access token..

If it is a **web application** then gclientauth will attempt to run a local
webserver to get the code itself and create a token so the user don't have to
do anything themselves.


## Example Usage:

```go
package main

func main() {
    scopes := []string{youtube.YoutubeReadonlyScope}
    ctx := oauth2.NoContext
    token, config, err := gclientauth.GetGoogleOauth2Token(ctx, client_secret, accesstoken, scopes, false, "8080")
    ...
    cfg := config.Client(ctx, token)
    ...
    gp, err := googlephotos.New(cfg)
    ...
    res, err := gp.Albums.List().Do()
    for _, a := range res.Albums {
        fmt.Printf("%v\n", a.Title)
    }
}
```
