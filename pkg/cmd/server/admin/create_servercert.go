package admin

import (
	"errors"
	"fmt"

	"github.com/spf13/cobra"
	"k8s.io/klog"

	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	kcmdutil "k8s.io/kubernetes/pkg/kubectl/cmd/util"
	"k8s.io/kubernetes/pkg/kubectl/util/templates"

	"github.com/openshift/library-go/pkg/crypto"
)

const CreateServerCertCommandName = "create-server-cert"

type CreateServerCertOptions struct {
	SignerCertOptions *SignerCertOptions

	CertFile string
	KeyFile  string

	ExpireDays int

	Hostnames []string
	Overwrite bool

	genericclioptions.IOStreams
}

var createServerLong = templates.LongDesc(`
	Create a key and server certificate

	Create a key and server certificate valid for the specified hostnames,
	signed by the specified CA. These are useful for securing infrastructure
	components such as the router, authentication server, etc.

	Example: Creating a secure router certificate.

	    CA=openshift.local.config/master
			%[1]s --signer-cert=$CA/ca.crt \
		          --signer-key=$CA/ca.key --signer-serial=$CA/ca.serial.txt \
		          --hostnames='*.cloudapps.example.com' \
		          --cert=cloudapps.crt --key=cloudapps.key
	    cat cloudapps.crt cloudapps.key $CA/ca.crt > cloudapps.router.pem
	`)

func NewCreateServerCertOptions(streams genericclioptions.IOStreams) *CreateServerCertOptions {
	return &CreateServerCertOptions{
		SignerCertOptions: NewDefaultSignerCertOptions(),
		ExpireDays:        crypto.DefaultCertificateLifetimeInDays,
		Overwrite:         true,
		IOStreams:         streams,
	}
}

func NewCommandCreateServerCert(commandName string, fullName string, streams genericclioptions.IOStreams) *cobra.Command {
	o := NewCreateServerCertOptions(streams)
	cmd := &cobra.Command{
		Use:   commandName,
		Short: "Create a signed server certificate and key",
		Long:  fmt.Sprintf(createServerLong, fullName),
		Run: func(cmd *cobra.Command, args []string) {
			kcmdutil.CheckErr(o.Validate(args))
			if _, err := o.CreateServerCert(); err != nil {
				kcmdutil.CheckErr(err)
			}
		},
	}

	BindSignerCertOptions(o.SignerCertOptions, cmd.Flags(), "")

	cmd.Flags().StringVar(&o.CertFile, "cert", o.CertFile, "The certificate file. Choose a name that indicates what the service is.")
	cmd.Flags().StringVar(&o.KeyFile, "key", o.KeyFile, "The key file. Choose a name that indicates what the service is.")

	cmd.Flags().StringSliceVar(&o.Hostnames, "hostnames", o.Hostnames, "Every hostname or IP you want server certs to be valid for. Comma delimited list")
	cmd.Flags().BoolVar(&o.Overwrite, "overwrite", o.Overwrite, "Overwrite existing cert files if found.  If false, any existing file will be left as-is.")

	cmd.Flags().IntVar(&o.ExpireDays, "expire-days", o.ExpireDays, "Validity of the certificate in days (defaults to 2 years). WARNING: extending this above default value is highly discouraged.")

	// autocompletion hints
	cmd.MarkFlagFilename("cert")
	cmd.MarkFlagFilename("key")

	return cmd
}

func (o CreateServerCertOptions) Validate(args []string) error {
	if len(args) != 0 {
		return errors.New("no arguments are supported")
	}
	if len(o.Hostnames) == 0 {
		return errors.New("at least one hostname must be provided")
	}
	if len(o.CertFile) == 0 {
		return errors.New("cert must be provided")
	}
	if len(o.KeyFile) == 0 {
		return errors.New("key must be provided")
	}

	if o.ExpireDays <= 0 {
		return errors.New("expire-days must be valid number of days")
	}

	if o.SignerCertOptions == nil {
		return errors.New("signer options are required")
	}
	if err := o.SignerCertOptions.Validate(); err != nil {
		return err
	}

	return nil
}

func (o CreateServerCertOptions) CreateServerCert() (*crypto.TLSCertificateConfig, error) {
	klog.V(4).Infof("Creating a server cert with: %#v", o)

	signerCert, err := o.SignerCertOptions.CA()
	if err != nil {
		return nil, err
	}

	var ca *crypto.TLSCertificateConfig
	written := true
	if o.Overwrite {
		ca, err = signerCert.MakeAndWriteServerCert(o.CertFile, o.KeyFile, sets.NewString([]string(o.Hostnames)...), o.ExpireDays)
	} else {
		ca, written, err = signerCert.EnsureServerCert(o.CertFile, o.KeyFile, sets.NewString([]string(o.Hostnames)...), o.ExpireDays)
	}
	if written {
		klog.V(3).Infof("Generated new server certificate as %s, key as %s\n", o.CertFile, o.KeyFile)
	} else {
		klog.V(3).Infof("Keeping existing server certificate at %s, key at %s\n", o.CertFile, o.KeyFile)
	}
	return ca, err
}
