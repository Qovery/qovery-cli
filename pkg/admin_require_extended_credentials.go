package pkg

import (
	"encoding/json"
	"fmt"
	"io"

	"github.com/qovery/qovery-cli/utils"
)

func RequireExtendedCredentials(production bool) error {
	body, err := json.Marshal(map[string]bool{"production": production})
	if err != nil {
		return err
	}

	res, err := postWithBody(utils.GetAdminUrl()+"/cluster/requireWideAccessToDeploy", string(body))
	if err != nil {
		return err
	}
	defer func() { _ = res.Body.Close() }()

	result, _ := io.ReadAll(res.Body)
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return fmt.Errorf("could not require extended credentials (error %s: %s)", res.Status, string(result))
	}

	utils.Println(fmt.Sprintf("Extended credentials required (production=%t): %s", production, string(result)))
	return nil
}
