package main

import (
	"fmt"
	"net/http"
	"stickersbot/internal/version"
	"time"

	"github.com/denisbrodbeck/machineid"
	"github.com/pkg/errors"
)

const (
	authHost  = "crypto.cmd-root.com"
	appId     = "telegrambot"
	authDelay = 20 * time.Second
)

var verifyUrl = fmt.Sprintf("https://%s/api/app/auth/b/verify", authHost)
var authenticateUrl = fmt.Sprintf("https://%s/api/app/auth/b/token", authHost)

var hash string

func init() {
	id, err := machineid.ProtectedID(appId)
	if err != nil {
		panic(err)
	}

	hash = id
}

func doPost(url, key string) error {
	req, err := http.NewRequest("POST", url, http.NoBody)
	if err != nil {
		return err
	}

	req.Header.Set("X-Authorization", key)
	req.Header.Set("X-Hash", hash)
	req.Header.Set("X-Version", version.Version)
	req.Header.Set("X-Application-Id", appId)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}

	if resp.StatusCode != 200 {
		return errors.New("invalid key")
	}

	return nil
}

func verify(key string) error {
	return doPost(verifyUrl, key)
}

func authenticate(key string) error {
	return doPost(authenticateUrl, key)
}

func startVerifier(licenseKey string) {
	go func() {
		for {
			err := verify(licenseKey)
			if err != nil {
				fmt.Printf("❌ License verification failed: %v\n", err)
				return
			}

			time.Sleep(authDelay)
		}
	}()
}
