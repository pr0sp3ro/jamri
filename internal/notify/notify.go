// Package notify sends desktop alerts.
package notify

import (
	"errors"
	"os"
	"os/exec"
)

const alarmSound = "/usr/share/sounds/freedesktop/stereo/alarm-clock-elapsed.oga"

// Alarm shows a desktop notification and plays an alarm sound. Commands are
// started without waiting, so the sound does not delay the caller. Their
// output goes to /dev/null.
func Alarm(summary, body string) error {
	notifyErr := exec.Command("notify-send", "--app-name=jamri", "--", summary, body).Start()

	return errors.Join(notifyErr, playAlarmSound())
}

func playAlarmSound() error {
	if _, err := os.Stat(alarmSound); err != nil {
		return err
	}

	err := exec.Command("pw-play", alarmSound).Start()

	if errors.Is(err, exec.ErrNotFound) {
		err = exec.Command("paplay", alarmSound).Start()
	}

	return err
}
