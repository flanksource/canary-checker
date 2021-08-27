package controllers

import v1 "github.com/flanksource/canary-checker/api/v1"

func getAllCheckKeys(canary *v1.Canary) (checkKeys []string) {
	checks := canary.Spec.GetAllChecks()
	for _, check := range checks {
		checkKeys = append(checkKeys, canary.GetKey(check))
	}
	return checkKeys
}

func contains(list []string, s string) bool {
	for _, element := range list {
		if element == s {
			return true
		}
	}
	return false
}
