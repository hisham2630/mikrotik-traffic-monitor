package models

import "testing"

func TestValidateTelegramBotToken(t *testing.T) {
	if err := ValidateTelegramBotToken("AAHxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx"); err == nil {
		t.Fatal("expected error for token without bot id prefix")
	}
	if err := ValidateTelegramBotToken("123456789:AAHxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx"); err != nil {
		t.Fatal(err)
	}
}

func TestValidateRuleNotifyChannelsRequiresOne(t *testing.T) {
	dir := t.TempDir()
	db, err := Open(dir+"/t.db", "")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	if err := db.ValidateRuleNotifyChannels(AlertRuleInput{}); err == nil {
		t.Fatal("expected error when no channel selected")
	}
}
