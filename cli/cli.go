package cli

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/gclaussn/go-bpmn/engine"
	"github.com/gclaussn/go-bpmn/http/client"
	"github.com/gclaussn/go-bpmn/http/common"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

const (
	envLookupAllowed = "envLookupAllowed" // flag level annotation that allows an environment variable lookup
	envPrefix        = "GO_BPMN_"
	noEngineRequired = "noEngineRequired" // annotation, indicating that no engine is required to run the command
	program          = "go-bpmn"

	envAuthorization         = envPrefix + "AUTHORIZATION"
	envHttpBasicAuthUsername = envPrefix + "HTTP_BASIC_AUTH_USERNAME"
	envHttpBasicAuthPassword = envPrefix + "HTTP_BASIC_AUTH_PASSWORD"
)

func New(version string) *Cli {
	cli := Cli{version: version}

	cli.rootCmd = newRootCmd(&cli)

	return &cli
}

type Cli struct {
	version string

	rootCmd *cobra.Command

	e            engine.Engine
	debugEnabled bool
	workerId     string
}

func (c *Cli) Execute() int {
	if err := c.rootCmd.Execute(); err != nil {
		return 1
	}
	return 0
}

func (c *Cli) help(cmd *cobra.Command, args []string) error {
	return cmd.Help()
}

func newRootCmd(cli *Cli) *cobra.Command {
	var (
		url     string
		timeout time.Duration
	)

	c := cobra.Command{
		Use:   program,
		Short: "A client for go-bpmn HTTP servers",
		PersistentPreRunE: func(c *cobra.Command, _ []string) error {
			c.SilenceUsage = true

			if _, ok := c.Annotations[noEngineRequired]; ok {
				return nil
			}

			if cli.e != nil {
				return nil // skip client creation when testing
			}

			c.Flags().VisitAll(func(f *pflag.Flag) {
				if f.Changed {
					return
				}
				if _, ok := f.Annotations[envLookupAllowed]; !ok {
					return
				}

				// e.g. worker-id -> GO_BPMN_WORKER_ID
				key := envPrefix + strings.ReplaceAll(strings.ToUpper(f.Name), "-", "_")

				if value, ok := os.LookupEnv(key); ok {
					f.Value.Set(value)
				}
			})

			authorization := os.Getenv(envAuthorization)
			if authorization == "" {
				username := os.Getenv(envHttpBasicAuthUsername)
				password := os.Getenv(envHttpBasicAuthPassword)

				if username == "" || password == "" {
					return fmt.Errorf(
						"no authorization set.\n\nfor pg:  use environment variable %s\nfor mem: use environment variable %s and %s\n ",
						envAuthorization,
						envHttpBasicAuthUsername,
						envHttpBasicAuthPassword,
					)
				}

				usernamePassword := fmt.Sprintf("%s:%s", username, password)
				authorization = "Basic " + base64.StdEncoding.EncodeToString([]byte(usernamePassword))
			}

			e, err := client.New(url, authorization, func(o *client.Options) {
				o.Timeout = timeout

				if cli.debugEnabled {
					o.OnRequest = debugRequest
					o.OnResponse = debugResponse
				}
			})
			if err != nil {
				return fmt.Errorf("failed to create HTTP client: %v", err)
			}

			cli.e = e
			return nil
		},
		RunE: cli.help,
		PersistentPostRun: func(cmd *cobra.Command, args []string) {
			if cli.e != nil {
				cli.e.Shutdown()
			}
		},
		Annotations: map[string]string{noEngineRequired: ""},
	}

	c.PersistentFlags().StringVar(&url, "url", "", "HTTP server URL")
	c.PersistentFlags().StringVar(&cli.workerId, "worker-id", program, "Worker ID")
	c.PersistentFlags().DurationVar(&timeout, "timeout", 40*time.Second, "Time limit for requests made by the HTTP client")
	c.PersistentFlags().BoolVar(&cli.debugEnabled, "debug", false, "Log HTTP requests and responses")

	c.PersistentFlags().SetAnnotation("url", envLookupAllowed, nil)
	c.PersistentFlags().SetAnnotation("worker-id", envLookupAllowed, nil)
	c.PersistentFlags().SetAnnotation("timeout", envLookupAllowed, nil)
	c.PersistentFlags().SetAnnotation("debug", envLookupAllowed, nil)

	c.AddCommand(newElementCmd(cli))
	c.AddCommand(newElementInstanceCmd(cli))
	c.AddCommand(newEventCmd(cli))
	c.AddCommand(newIncidentCmd(cli))
	c.AddCommand(newJobCmd(cli))
	c.AddCommand(newProcessCmd(cli))
	c.AddCommand(newProcessInstanceCmd(cli))
	c.AddCommand(newTaskCmd(cli))
	c.AddCommand(newVariableCmd(cli))
	c.AddCommand(newSetTimeCmd(cli))
	c.AddCommand(newVersionCmd(cli))

	return &c
}

func newSetTimeCmd(cli *Cli) *cobra.Command {
	var (
		timeV timeValue

		cmd engine.SetTimeCmd
	)

	c := cobra.Command{
		Use:   "set-time",
		Short: "Set the engine's time",
		RunE: func(c *cobra.Command, _ []string) error {
			cmd.Time = time.Time(timeV)

			return cli.e.SetTime(context.Background(), cmd)
		},
	}

	c.Flags().Var(&timeV, "time", "A future point in time")

	return &c
}

func newVersionCmd(cli *Cli) *cobra.Command {
	c := cobra.Command{
		Use:   "version",
		Short: "Show version",
		Run: func(c *cobra.Command, _ []string) {
			c.Println(cli.version)
		},
		Annotations: map[string]string{noEngineRequired: ""},
	}

	return &c
}

func debugRequest(req *http.Request) error {
	log.Printf("%s %s", req.Method, req.URL)

	if req.Body == nil {
		return nil
	}

	b, err := io.ReadAll(req.Body)
	if err != nil {
		return err
	}

	var reqBodyStr string

	buf := &bytes.Buffer{}
	if err := json.Indent(buf, b, "", "  "); err != nil {
		reqBodyStr = string(b)
	} else {
		reqBodyStr = buf.String()
	}

	req.Body = io.NopCloser(bytes.NewReader(b)) // make body readable again

	log.Printf("request body:\n%s", reqBodyStr)
	return nil
}

func debugResponse(res *http.Response) error {
	log.Printf("status code: %d", res.StatusCode)

	log.Println("response headers:")
	for name, values := range res.Header {
		log.Printf("%s: %s", name, strings.Join(values, ", "))
	}

	resBody := res.Body
	defer resBody.Close()

	b, err := io.ReadAll(resBody)
	if err != nil {
		log.Printf("failed to read response body: %v", err)
		return err
	}

	res.Body = nil

	var resBodyStr string

	contentType := res.Header.Get(common.HeaderContentType)
	if contentType == common.ContentTypeJson || contentType == common.ContentTypeProblemJson {
		buf := &bytes.Buffer{}
		if err := json.Indent(buf, b, "", "  "); err == nil {
			resBodyStr = buf.String()
			res.Body = io.NopCloser(buf) // make body readable again
		}
	}

	if res.Body == nil {
		resBodyStr = string(b)
		res.Body = io.NopCloser(bytes.NewReader(b)) // make body readable again
	}

	if resBodyStr != "" && contentType != common.ContentTypeXml {
		log.Printf("response body:\n%s", resBodyStr)
	}
	return nil
}
