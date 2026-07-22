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

	"github.com/gclaussn/go-bpmn/cli/common"
	"github.com/gclaussn/go-bpmn/cli/element"
	"github.com/gclaussn/go-bpmn/cli/element_instance"
	"github.com/gclaussn/go-bpmn/cli/incident"
	"github.com/gclaussn/go-bpmn/cli/job"
	"github.com/gclaussn/go-bpmn/cli/message"
	"github.com/gclaussn/go-bpmn/cli/process"
	"github.com/gclaussn/go-bpmn/cli/process_instance"
	"github.com/gclaussn/go-bpmn/cli/signal"
	"github.com/gclaussn/go-bpmn/cli/task"
	"github.com/gclaussn/go-bpmn/cli/user_task"
	"github.com/gclaussn/go-bpmn/cli/variable"
	"github.com/gclaussn/go-bpmn/engine"
	"github.com/gclaussn/go-bpmn/http/client"
	httpcommon "github.com/gclaussn/go-bpmn/http/common"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

const (
	envLookupAllowed = "envLookupAllowed" // flag level annotation that allows an environment variable lookup
	envPrefix        = "GO_BPMN_"

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
}

func (c *Cli) RootCmd() *cobra.Command {
	return c.rootCmd
}

func newRootCmd(cli *Cli) *cobra.Command {
	var (
		debugEnabled bool
		timeout      time.Duration
		url          string
		workerId     string
	)

	c := cobra.Command{
		Use:   "go-bpmn",
		Short: "A client for go-bpmn HTTP servers",
		PersistentPreRunE: func(c *cobra.Command, _ []string) error {
			c.SilenceUsage = true

			if _, ok := c.Annotations[common.AnnotationEngine]; !ok {
				return nil
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

				if debugEnabled {
					o.OnRequest = debugRequest
					o.OnResponse = debugResponse
				}
			})
			if err != nil {
				return fmt.Errorf("failed to create HTTP client: %v", err)
			}

			c.SetContext(common.SetEngineAndWorkerId(c.Context(), e, workerId))

			return nil
		},
		RunE: common.Help,
		PersistentPostRun: func(c *cobra.Command, args []string) {
			if e := common.GetEngine(c); e != nil {
				e.Shutdown()
			}
		},
	}

	c.PersistentFlags().StringVar(&url, "url", "", "HTTP server URL")
	c.PersistentFlags().StringVar(&workerId, "worker-id", "go-bpmn", "Worker ID")
	c.PersistentFlags().DurationVar(&timeout, "timeout", 40*time.Second, "Time limit for requests made by the HTTP client")
	c.PersistentFlags().BoolVar(&debugEnabled, "debug", false, "Log HTTP requests and responses")

	c.PersistentFlags().SetAnnotation("url", envLookupAllowed, nil)
	c.PersistentFlags().SetAnnotation("worker-id", envLookupAllowed, nil)
	c.PersistentFlags().SetAnnotation("timeout", envLookupAllowed, nil)
	c.PersistentFlags().SetAnnotation("debug", envLookupAllowed, nil)

	c.AddCommand(
		element.NewCmd(),
		element_instance.NewCmd(),
		incident.NewCmd(),
		job.NewCmd(),
		message.NewCmd(),
		process.NewCmd(),
		process_instance.NewCmd(),
		signal.NewCmd(),
		task.NewCmd(),
		user_task.NewCmd(),
		variable.NewCmd(),
		newSetTimeCmd(),
		newVersionCmd(cli),
	)

	c.SetHelpCommand(&cobra.Command{
		Use:    "no-help",
		Hidden: true,
	})

	c.CompletionOptions.HiddenDefaultCmd = true

	c.PersistentFlags().BoolP("help", "h", false, "")
	c.PersistentFlags().Lookup("help").Hidden = true

	return &c
}

func newSetTimeCmd() *cobra.Command {
	format := time.RFC3339

	var timer common.Timer

	c := cobra.Command{
		Use:         "set-time",
		Short:       "Increases the engine's time for testing purposes",
		Annotations: common.AnnotationEngineMap,
		RunE: func(c *cobra.Command, _ []string) error {
			e := common.GetEngine(c)

			new, old, err := e.SetTime(context.Background(), engine.SetTimeCmd{
				Time:         timer.Time.Time(),
				TimeCycle:    timer.TimeCycle,
				TimeDuration: engine.ISO8601Duration(timer.TimeDuration),
			})
			if err != nil {
				return err
			}

			c.Printf("New time: %s\nOld time: %s", new.Format(format), old.Format(format))
			return nil
		},
	}

	c.Flags().Var(&timer.Time, "time", "A future point in time")
	c.Flags().StringVar(&timer.TimeCycle, "time-cycle", "", "CRON expression, when evaluated the next tick specifies the engine's new time")
	c.Flags().Var(&timer.TimeDuration, "time-duration", "Duration, used to calculate a future point in time")

	return &c
}

func newVersionCmd(cli *Cli) *cobra.Command {
	c := cobra.Command{
		Use:   "version",
		Short: "Show version",
		Run: func(c *cobra.Command, _ []string) {
			c.Println(cli.version)
		},
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

	contentType := res.Header.Get(httpcommon.HeaderContentType)
	if contentType == httpcommon.ContentTypeJson || contentType == httpcommon.ContentTypeProblemJson {
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

	if resBodyStr != "" && contentType != httpcommon.ContentTypeXml {
		log.Printf("response body:\n%s", resBodyStr)
	}
	return nil
}
