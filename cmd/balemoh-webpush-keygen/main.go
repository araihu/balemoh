// balemoh-webpush-keygen writes a VAPID key without exposing it on stdout.
package main

import (
	"fmt"
	protocol "github.com/SherClockHolmes/webpush-go"
	"os"
)

func main() {
	if err := generate(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
func generate(args []string) error {
	if len(args) != 1 {
		return fmt.Errorf("usage: balemoh-webpush-keygen <new-private-key-file>")
	}
	privateKey, publicKey, err := protocol.GenerateVAPIDKeys()
	if err != nil {
		return fmt.Errorf("generate VAPID key failed")
	}
	file, err := os.OpenFile(args[0], os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return fmt.Errorf("create new private-key file failed")
	}
	if _, err = file.WriteString(privateKey + "\n"); err != nil {
		file.Close()
		os.Remove(args[0])
		return fmt.Errorf("write private-key file failed")
	}
	if err = file.Close(); err != nil {
		return fmt.Errorf("close private-key file failed")
	}
	fmt.Println("Public application server key:", publicKey)
	return nil
}
