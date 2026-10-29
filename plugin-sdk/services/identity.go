// Package services contains data-only host service identity validation.
package services

import "regexp"

var idPattern = regexp.MustCompile(`^[a-f0-9]{64}$`)

func ValidID(id string) bool { return idPattern.MatchString(id) }
