package iam

import (
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/iam"
	"github.com/aws/aws-sdk-go/service/iam/iamiface"
	log "github.com/sirupsen/logrus"
)

type PolicyDocument struct {
	Version   string
	Statement []StatementEntry
}

type StatementEntry struct {
	Sid       string `json:",omitempty"`
	Effect    string
	Action    []string
	Resource  string
	Condition Condition `json:",omitempty"`
}

// Condition maps a condition operator to the condition-key/condition-value statement
// ie. "{ "StringEquals" : { "aws:username" : "johndoe" }}"
// for more information, see https://docs.aws.amazon.com/IAM/latest/UserGuide/reference_policies_elements_condition.html
type Condition map[string]ConditionStatement

// ConditionStatement maps condition-key to condition-value
// ie. "{ "aws:username" : "johndoe" }"
type ConditionStatement map[string]string

type IAM struct {
	session *session.Session
	Service iamiface.IAMAPI
}

type IAMOption func(*IAM)

func New(opts ...IAMOption) IAM {
	i := IAM{}

	for _, opt := range opts {
		opt(&i)
	}

	if i.session != nil {
		i.Service = iam.New(i.session)
	}

	return i
}

func WithSession(sess *session.Session) IAMOption {
	return func(i *IAM) {
		log.Debug("using aws session")
		i.session = sess
	}
}

func WithCredentials(key, secret, token, region string) IAMOption {
	return func(i *IAM) {
		log.Debugf("creating new session with key id %s in region %s", key, region)
		sess := session.Must(session.NewSession(&aws.Config{
			Credentials: credentials.NewStaticCredentials(key, secret, token),
			Region:      aws.String(region),
		}))
		i.session = sess
	}
}
