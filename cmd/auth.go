package cmd

import (
	"context"
	"fmt"
	"github.com/pkg/browser"
	"github.com/spf13/cobra"
	"net/http"
	"qovery.go/api"
	"time"
)

var authCmd = &cobra.Command{
	Use:   "auth",
	Short: "Do authentication",
	Long: `AUTH do auth on Qovery service. For example:

	qovery auth`,
	Run: func(cmd *cobra.Command, args []string) {
		// TODO link to web auth
		_ = browser.OpenURL("https://auth.qovery.com/login?client=MJ2SJpu12PxIzgmc5z5Y7N8m5MnaF7Y0" +
			"&protocol=oauth2&response_type=id_token%20token&audience=https://core.qovery.com&redirect_uri=" +
			"http%3A%2F%2Flocalhost:10999/authorization")

		fmt.Println("Waiting for authentication...")

		srv := &http.Server{Addr: "localhost:10999"}

		http.HandleFunc("/authorization", func(writer http.ResponseWriter, request *http.Request) {
			js := `<script type="text/javascript" charset="utf-8">
				var hash = window.location.hash.split("=")[1].split("&")[0];
				var xmlHttp = new XMLHttpRequest();
				xmlHttp.open("GET", "http://localhost:10999/authorization/valid?access_token=" + hash, false);
				xmlHttp.send(null);
				xmlHttp.responseText;
             </script>`

			_, _ = writer.Write([]byte(js))
			_, _ = writer.Write([]byte("Authentication successful. You can close this window."))
		})

		http.HandleFunc("/authorization/valid", func(writer http.ResponseWriter, request *http.Request) {

			accessToken := request.URL.Query()["access_token"][0]

			api.SetAuthorizationToken(accessToken)

			accountId := api.GetAccount().Id
			if accountId != "" {
				api.SetAccountId(accountId)
				fmt.Println("Authentication successful!")
			}

			go func() {
				time.Sleep(time.Duration(1) * time.Second)
				_ = srv.Shutdown(context.TODO())
			}()
		})

		_ = srv.ListenAndServe()
	},
}

func init() {
	RootCmd.AddCommand(authCmd)
}
