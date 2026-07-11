package daemon

import (
	"fmt"
	"slices"
	"strings"

	"github.com/gclaussn/go-bpmn/engine"
	"github.com/spf13/cobra"
)

const (
	AnnotationConf      = "conf"      // annotates commands that require a configuration
	AnnotationValidConf = "validConf" // annotates commands that require a valid configuration
)

func Help(cmd *cobra.Command, _ []string) error {
	return cmd.Help()
}

func NewCreateEncryptionKeyCmd() *cobra.Command {
	c := cobra.Command{
		Use:   "create-encryption-key",
		Short: "Create a new encryption key - used for " + envPrefix + optEncryptionKeys,
		RunE: func(c *cobra.Command, _ []string) error {
			encryptionKey, err := engine.NewEncryptionKey()
			if err != nil {
				return fmt.Errorf("failed to create encryption key: %v", err)
			}

			c.Print(encryptionKey)
			return nil
		},
	}

	return &c
}

func NewListConfCmd(conf Conf) *cobra.Command {
	c := cobra.Command{
		Use:         "list-conf",
		Short:       "List configuration",
		Annotations: map[string]string{AnnotationConf: ""},
		Run: func(c *cobra.Command, _ []string) {
			opts := make([]ConfOpt, len(conf.opts))

			i := 0
			for _, opt := range conf.opts {
				opts[i] = opt
				i++
			}

			slices.SortFunc(opts, func(a ConfOpt, b ConfOpt) int {
				return strings.Compare(a.key, b.key)
			})

			for _, opt := range opts {
				c.Printf("%s=%s\n", opt.key, opt.Value())
			}
		},
	}

	return &c
}

func NewListConfOptsCmd(conf Conf) *cobra.Command {
	c := cobra.Command{
		Use:         "list-conf-opts",
		Short:       "List configuration options",
		Annotations: map[string]string{AnnotationConf: ""},
		Run: func(c *cobra.Command, _ []string) {
			opts := make([]ConfOpt, len(conf.opts))

			i := 0
			for _, opt := range conf.opts {
				opts[i] = opt
				i++
			}

			slices.SortFunc(opts, func(a ConfOpt, b ConfOpt) int {
				return strings.Compare(a.key, b.key)
			})

			maxKeyLength := 0
			for _, opt := range opts {
				keyLength := len(opt.key)
				if opt.required {
					keyLength++
				}

				if keyLength > maxKeyLength {
					maxKeyLength = keyLength
				}
			}

			var sb strings.Builder
			for _, opt := range opts {
				sb.WriteString(opt.key)

				l := len(opt.key)
				if opt.required {
					sb.WriteRune('*')
					l++
				}

				sb.WriteString(strings.Repeat(" ", maxKeyLength-l))
				sb.WriteString("   ")
				sb.WriteString(opt.description)

				if opt.defaultValue != "" {
					fmt.Fprintf(&sb, " - default: %s", opt.defaultValue)
				}

				sb.WriteRune('\n')
			}

			c.Print(sb.String())
		},
	}

	return &c
}

func NewVersionCmd(version string) *cobra.Command {
	c := cobra.Command{
		Use:   "version",
		Short: "Show version",
		Run: func(c *cobra.Command, _ []string) {
			c.Println(version)
		},
	}

	return &c
}
