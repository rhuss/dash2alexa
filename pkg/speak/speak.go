package speak

import (
	"log"
	"runtime"
	"os/exec"
	"fmt"
	"strings"
)

// Speak converts a text to audio and the send it out via audio
func Speak(text string, options *Options) error {
	if options.Backend == "ivona" {
		return IvonaSpeak(text, options)
	}
	if options.Backend == "polly" {
		return PollySpeak(text, options)
	}

	log.Printf(">>> Unknown backend %s. Ignoring", options.Backend)
	return nil
}

func getPlayCommand(player, mp3 string) *exec.Cmd {
	if player != "" {
		parts := strings.Split(fmt.Sprintf(player, mp3), " ")
		return exec.Command(parts[0], parts[1:]...)
	}
	// Check for default:
	if runtime.GOOS == "darwin" {
		return exec.Command("afplay", mp3)
	}
	return exec.Command("mpg123", mp3)
}

// Options are used for configuring the way how to create the speech output
type Options struct {
	Access   string
	Secret   string
	Gender   string
	Language string
	Backend  string
	Player   string
}
