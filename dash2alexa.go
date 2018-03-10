// Copyright © 2016 NAME HERE <EMAIL ADDRESS>
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package main

import (
	"github.com/spf13/viper"
	"github.com/spf13/cobra"
	"fmt"
	"os"
	"github.com/rhuss/dash"
	"github.com/rhuss/dash2alexa/pkg/speak"
	"errors"
	"log"
	"net"
)

var cfgFile string
var gender string
var language string
var backend string

// RootCmd represents the base command when called without any subcommands
var RootCmd = &cobra.Command{
	Use:   "dash2alexa",
	Short: "Trigger Alexa by pressing Dash buttons",
	Long: `dash2alexa: Trigger Amazon Alexa with Amazon Dash buttons.

This utility tool will watch ARP traffic for certain MAC addresses and sends
a list of messages via the audio interface, which potential triggers Amazon Alexa
	`,
	Run: watch,
}

// Execute adds all child messages to the root command sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func main() {
	if err := RootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(-1)
	}
}

func init() {
	cobra.OnInitialize(initConfig)

	RootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is $HOME/.dash2alexa)")
	RootCmd.PersistentFlags().StringVarP(&gender, "gender", "g", "female", "Gender of voice to use (male or female)")
	RootCmd.PersistentFlags().StringVarP(&language, "language", "l", "de", "Language to use ('de' or 'en')")
	RootCmd.PersistentFlags().StringVarP(&backend, "backend", "b", "polly", "Service type ('ivona' or 'polly')")
}

func initConfig() {
	if cfgFile != "" { // enable ability to specify config file via flag
		viper.SetConfigFile(cfgFile)
	}

	viper.SetConfigName(".dash2alexa") // name of config file (without extension)
	viper.AddConfigPath("$HOME/")      // adding home directory as first search path
	viper.AutomaticEnv()               // read in environment variables that match

	viper.BindPFlag("gender", RootCmd.PersistentFlags().Lookup("gender"))
	viper.BindPFlag("language", RootCmd.PersistentFlags().Lookup("language"))
	viper.BindPFlag("backend", RootCmd.PersistentFlags().Lookup("backend"))

	// If a config file is found, read it in.
	if err := viper.ReadInConfig(); err == nil {
		fmt.Println("Using config file:", viper.ConfigFileUsed())
	}
}

// ================================================================================================

type alexaCommand struct {
	name     string
	mac      string
	wait     int
	messages []string
}

func watch(cmd *cobra.Command, args []string) {
	netInterface := viper.GetString("interface")
	if netInterface == "" {
		netInterface = "en3"
	}

	iface, err := net.InterfaceByName(netInterface)
	if err != nil {
		panic(err)
	}

	addr, err := extractAddress(iface)
	if err != nil {
		panic(err)
	}

	var buttonChans = [] *chan dash.ButtonEvent{}
	var buttonCommands = make(map[string]alexaCommand)

	buttons := viper.Get("buttons").([]map[string]interface{})
	for _, button := range buttons {
		mac := button["mac"].(string)
		if mac == "" {
			panic("No mac given for button configuration")
		}
		wait := button["wait"].(int)
		if wait == 0 {
			wait = 4
		}
		buttonCommands[mac] = alexaCommand{
			name:     button["name"].(string),
			mac:      mac,
			wait:     wait,
			messages: button["messages"].([]string),
		}
		buttonChans = append(buttonChans, dash.WatchButton(iface, mac))
	}

	agg := make(chan dash.ButtonEvent)
	for _, ch := range buttonChans {
		go func(c chan dash.ButtonEvent) {
			for event := range c {
				agg <- event
			}
		}(*ch)
	}

	log.Printf("Using network range %v for interface %v", addr, iface.Name)

	for {
		select {
		case buttonEvent := <-agg:
			callAlexa(buttonCommands[buttonEvent.MacAddress])
		}
	}
}

// speakOptions create the options for the text to speech service
func speakOptions() *speak.Options {
	speechConfig := viper.GetStringMapString("backend")
	if speechConfig == nil {
		log.Fatal("No authentication for speech backend configured")
	}
	access := viper.GetString("access")
	if access == "" {
		log.Fatal("No access for speech backend found")
	}
	secret := viper.GetString("secret")
	if secret == "" {
		log.Fatal("No secret given for accessing speech backend")
	}

	return &speak.Options{
		Access:   access,
		Secret:   secret,
		Gender:   gender,
		Language: language,
		Backend:  backend,
	}
}

func callAlexa(command alexaCommand) {
	log.Print("Button pushed")
	for _, msg := range command.messages {
		speak.Speak(msg, speakOptions())

	}
}

func extractAddress(iface *net.Interface) (*net.IPNet, error) {
	var addr *net.IPNet
	if addrs, err := iface.Addrs(); err != nil {
		return nil, err
	} else {
		for _, a := range addrs {
			if ipnet, ok := a.(*net.IPNet); ok {
				if ip4 := ipnet.IP.To4(); ip4 != nil {
					addr = &net.IPNet{
						IP:   ip4,
						Mask: ipnet.Mask[len(ipnet.Mask)-4:],
					}
					break
				}
			}
		}
	}
	// Sanity-check that the interface has a good address.
	if addr == nil {
		return nil, errors.New("no good IP network found")
	} else if addr.IP[0] == 127 {
		return nil, errors.New("skipping localhost")
	} else if addr.Mask[0] != 0xff || addr.Mask[1] != 0xff {
		return nil, errors.New("mask means network is too large")
	}
	return addr, nil
}
