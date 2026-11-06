//go:build !windows

package steam

func setSteamAutoLoginRegistry(accountName string) error {
	return nil
}
