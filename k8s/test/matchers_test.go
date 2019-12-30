package k8s_test

import (
  "fmt"
	"os/exec"
	. "github.com/onsi/ginkgo"
	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/gexec"
	"github.com/onsi/gomega/format"
	"github.com/onsi/gomega/types"
	appV1 "k8s.io/api/apps/v1"
	coreV1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes/scheme"
)

type RenderingContext struct {
  templates []string
  data map[string]string
}

func (r RenderingContext) WithData(data map[string]string) RenderingContext {
  r.data = data
  return r
}

func NewRenderingContext(templates ...string) RenderingContext {
  return RenderingContext { templates, nil }
}

type SatisfyTestOverlayMatcher struct {
  overlay string
}

func SatisfyTestOverlay(overlay string) *SatisfyTestOverlayMatcher {
  return &SatisfyTestOverlayMatcher { overlay }
}

func (p *SatisfyTestOverlayMatcher) Match(actual interface{}) (bool, error) {
	rendering, ok := actual.(RenderingContext)
	if !ok {
		return false, fmt.Errorf("SatisfyTestOverlay must be passed a RenderingContext. Got\n%s", format.Object(actual, 1))
	}

  session, err := renderWithData(append(rendering.templates, p.overlay), rendering.data)
  if err != nil {
    return false, err
  }

  if session.ExitCode() != 0 {
    return false, fmt.Errorf(string(session.Err.Contents()))
  }

  return true, nil
}

func (matcher *SatisfyTestOverlayMatcher) FailureMessage(actual interface{}) string {
	return "Expected TAML to match expectation"
}

func (matcher *SatisfyTestOverlayMatcher) NegatedFailureMessage(actual interface{}) string {
	return "Expected YAML not to match expectation"
}

type ProduceYAMLMatcher struct {
  matcher types.GomegaMatcher
}

func ProduceYAML(matcher types.GomegaMatcher) *ProduceYAMLMatcher {
  return &ProduceYAMLMatcher { matcher }
}

func (p *ProduceYAMLMatcher) Match(actual interface{}) (bool, error) {
	rendering, ok := actual.(RenderingContext)
	if !ok {
		return false, fmt.Errorf("ProduceYAML must be passed a RenderingContext. Got\n%s", format.Object(actual, 1))
	}

  session, err := renderWithData(rendering.templates, rendering.data)
  if err != nil {
    return false, err
  }

  obj, err := parseYAML(session.Out)
  if err != nil {
    return false, err
  }

  return p.matcher.Match(obj)
}

func (matcher *ProduceYAMLMatcher) FailureMessage(actual interface{}) string {
	return "Expected TAML to match expectation"
}

func (matcher *ProduceYAMLMatcher) NegatedFailureMessage(actual interface{}) string {
	return "Expected YAML not to match expectation"
}

func renderWithData(templates []string, data map[string]string) (*gexec.Session, error) {
  var args []string
  for _, template := range templates {
    args = append(args, "-f")
    args = append(args, template)
  }

  for k, v := range data {
    args = append(args, "-v")
    args = append(args, fmt.Sprintf("%s=%s", k, v))
  }

	//command := exec.Command("ytt", args...)
	command := exec.Command("/Users/shamus/projects/go/src/github.com/k14s/ytt/ytt", args...)
	session, err := gexec.Start(command, GinkgoWriter, GinkgoWriter)
  if err != nil {
    return session, err
  }

	return session.Wait(), nil
}

func parseYAML(yaml *gbytes.Buffer) (interface{}, error) {
	decode := scheme.Codecs.UniversalDeserializer().Decode
	obj, _, err := decode(yaml.Contents(), nil, nil)
  if err != nil {
    return nil, err
  }

	return obj, nil
}

type ContainerExpectation func(coreV1.Container) error

type HavingContainerMatcher struct {
  name string
  tests []ContainerExpectation
}

func HavingContainer(name string) *HavingContainerMatcher {
	return &HavingContainerMatcher{name, nil}
}

func (matcher *HavingContainerMatcher) RunningImage(image string) *HavingContainerMatcher {
  matcher.tests = append(matcher.tests, func(container coreV1.Container) error {
    if container.Image != image {
      return fmt.Errorf("Expected container to run image %s but instead it wil run %s", image, container.Image)
    }

    return nil
  })

  return matcher
}

func (matcher *HavingContainerMatcher) Match(actual interface{}) (bool, error) {
	deployment, ok := actual.(*appV1.Deployment)
	if !ok {
		return false, fmt.Errorf("HavingContainer must be passed a deployment. Got\n%s", format.Object(actual, 1))
	}

  var selected *coreV1.Container
  for _, c := range deployment.Spec.Template.Spec.Containers {
    if c.Name == matcher.name {
      selected = &c
    }
  }

  if selected == nil {
    return false, fmt.Errorf("Expected container named %s, but did not find one", matcher.name)
  }

  for _, test := range matcher.tests {
    if err := test(*selected); err != nil {
      return false, err;
    }
  }

  return true, nil
}

func (matcher *HavingContainerMatcher) FailureMessage(actual interface{}) string {
	return "Expected deployment to match expectation"
}

func (matcher *HavingContainerMatcher) NegatedFailureMessage(actual interface{}) string {
	return "Expected deployment not to match expectation"
}
