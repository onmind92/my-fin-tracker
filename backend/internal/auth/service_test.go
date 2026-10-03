package auth

import (
	"testing"
	"time"
)

func TestPasswordHashVerifyAndWrongPassword(t *testing.T) {
	hash, err := HashPassword("correct horse battery staple")
	if err != nil {
		t.Fatal(err)
	}
	if !VerifyPassword(hash, "correct horse battery staple") {
		t.Fatal("correct password rejected")
	}
	if VerifyPassword(hash, "wrong password") {
		t.Fatal("wrong password accepted")
	}
	if hash == "correct horse battery staple" {
		t.Fatal("password stored as plaintext")
	}
}
func TestExpiredSession(t *testing.T) {
	now := time.Now()
	if SessionValid(now.Add(-time.Second), now) {
		t.Fatal("expired session accepted")
	}
	if !SessionValid(now.Add(time.Second), now) {
		t.Fatal("valid session rejected")
	}
}
func TestNormalizeEmailAndTimezone(t *testing.T) {
	email, err := NormalizeEmail(" User@Example.COM ")
	if err != nil || email != "user@example.com" {
		t.Fatalf("%q %v", email, err)
	}
	if ValidateTimezone("Europe/Moscow") != nil {
		t.Fatal("valid timezone rejected")
	}
	if ValidateTimezone("Mars/Olympus") == nil {
		t.Fatal("invalid timezone accepted")
	}
}
