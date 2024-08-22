package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"sync"

	"github.com/manifoldco/promptui"
)

/** OpenAI submission parameters struct.  Example:
  "prompt": "Once upon a time",
  "max_tokens": 5,
  "temperature": 1,
  "top_p": 1,
  "n": 1,
  "stream": false,
  "logprobs": null,
  "stop": "\n"
**/

var api_key = os.Getenv("OPENAI_API_KEY")

const (
	//Preamble         = "Preamble text"
	Prefix           = "# Shell\n#"
	Postfix          = "\n$ "
	Stop             = "\n"
	Temperature      = 0
	MaxTokens        = 100
	TopP             = 1
	OpenAPIEndpoint  = "https://api.openai.com/v1/engines/davinci-codex/completions"
	FrequencyPenalty = 0
	PresencePenalty  = 0
)

type OpenAISubmission struct {
	Prompt      string  `json:"prompt"`
	Temperature float64 `json:"temperature"`
	MaxTokens   int     `json:"max_tokens"`
	TopP        float64 `json:"top_p"`
	//N           int     `json:"n"`
	//Stream           bool    `json:"stream"`
	//LogProbs         bool    `json:"logprobs"`
	Stop             string  `json:"stop"`
	FrequencyPenalty float64 `json:"frequency_penalty"`
	PresencePenalty  float64 `json:"presence_penalty"`
}

/** OpenAI Response struct. Example:
  "id": "cmpl-uqkvlQyYK7bGYrRHQ0eXlWi7",
  "object": "text_completion",
  "created": 1589478378,
  "model": "davinci-codex:2020-05-03",
  "choices": [
    {
      "text": " there was a girl who",
      "index": 0,
      "logprobs": null,
      "finish_reason": "length"
    }
  ]
**/
type OpenAIResponse struct {
	Id      string                 `json:"id"`
	Object  string                 `json:"object"`
	Created int                    `json:"created"`
	Model   string                 `json:"model"`
	Choices []OpenAIResponseChoice `json:"choices"`
}

/** Struct for openAI response choice. Example:
  "text": " there was a girl who",
  "index": 0,
  "logprobs": null,
  "finish_reason": "length"
**/
type OpenAIResponseChoice struct {
	Text         string `json:"text"`
	Index        int    `json:"index"`
	LogProbs     bool   `json:"logprobs"`
	FinishReason string `json:"finish_reason"`
}

type UserResponseParams struct {
	Prompt string
}

func OpenAISubmitWithTemp(submission OpenAISubmission, heatIndex int) OpenAIResponse {
	submission.Temperature = float64(heatIndex) * .2
	return OpenAISubmit(submission)
}

// OpenAISubmit Marshalls an OpenAISubmission into JSON, submits it to the OpenAPI Rest Endpoint, unmarshalls it into an OpenAIResponse object
func OpenAISubmit(submission OpenAISubmission) OpenAIResponse {
	payloadBuffer := new(bytes.Buffer)
	json.NewEncoder(payloadBuffer).Encode(submission)
	req, _ := http.NewRequest("POST", OpenAPIEndpoint, payloadBuffer)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+api_key)
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		panic(err)
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		panic(err)
	}
	// Marshal the JSON byte array into an OpenAIResponse object
	var response OpenAIResponse
	err = json.Unmarshal(body, &response)
	if err != nil {
		panic(err)
	}
	return response
}

func main() {
	// define flags
	var (
		context      = flag.String("context", "# Shell\n#", "Which context to use (may conflict with prompt and plugins)")
		promptString = flag.String("prompt", "$ ", "Set a custom prompt such as '>>> ' or 'msf> '")
		//interactive  = flag.Bool("i", true, "Set whether you get a selectable menu")
		//responses    = flag.Int("prompt", 3, "Set number of recommended options")
		//eli5         = flag.Bool("explain", true, "Set whether you get an explanation of each command")
		//scope      = flag.String("plugins", "", "Which scope(s) to use such as 'Metasploit' or 'Kubernetes,Docker'")
	)
	flag.Parse()
	args := os.Args[1:]
	prompt := strings.Join(args, " ")

	// Create an OpenAISubmission object
	submission := OpenAISubmission{
		Prompt:      *context + " " + prompt + " \n" + *promptString,
		MaxTokens:   MaxTokens,
		Temperature: Temperature,
		TopP:        TopP,
		//N:           N,
		//Stream:      Stream,
		//LogProbs:    LogProbs,
		Stop:             Stop,
		FrequencyPenalty: 0,
		PresencePenalty:  0,
	}
	if len(args) == 0 {
		MainMenu(submission)
		return
	}

	// Submit the OpenAISubmission object to the OpenAPI Endpoint
	if len(args) > 0 {
		response := OpenAISubmit(submission)
		fmt.Println(response.Choices[0].Text)
	}
}

func MainMenu(sub OpenAISubmission) error {
	var wg sync.WaitGroup
	var options []string
	selectables := make(map[string]int)

	validate := func(input string) error {
		if len(input) < 3 {
			return errors.New("too short")
		}
		return nil
	}

	prompt := promptui.Prompt{
		Label:    "What do you want to do?",
		Validate: validate,
	}

	result, err := prompt.Run()
	if err != nil {
		println("Error reading user response")
		return err
	}
	sub.Prompt = "# Shell\n#" + result + "\n" + "$ "

	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			//sub.Temperature = float64(i) * 0.2
			response := OpenAISubmitWithTemp(sub, idx)
			// if stringNotInSlice(options, response.Choices[0].Text) {
			// 	options = append(options, response.Choices[0].Text)
			// }
			if _, ok := selectables[response.Choices[0].Text]; !ok {
				selectables[response.Choices[0].Text] = idx
			}
			//println(selectables[response.Choices[0].Text])
		}(i)
	}
	wg.Wait()
	for i := 0; i < 5; i++ {
		for val, idx := range selectables {
			if idx == i {
				options = append(options, val)
			}
		}
	}
	options = append(options, "Quit")

	list := promptui.Select{
		Label: "Commands",
		Items: options,
	}

	_, value, _ := list.Run()

	if value == "Quit" {
		return nil
	}

	list = promptui.Select{
		Label: "Execute?",
		Items: []string{"Execute", "Print", "Quit"},
	}

	_, nextStep, _ := list.Run()

	switch nextStep {
	case "Execute":
		{
			out, err := exec.Command(os.Getenv("SHELL"), "-c", value).CombinedOutput()
			println(string(out))
			if err != nil {
				println("Error with commanding " + err.Error())
				return err
			}
		}
	}
	return nil
}

func Config() {
	println("not implemented yet")
	return
}

// func stringNotInSlice(list []string, a string) bool {
// 	for _, b := range list {
// 		if b == a {
// 			return false
// 		}
// 	}
// 	return true
// }
