package support

import (
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"time"

	v12 "k8s.io/api/apps/v1"
	v13 "k8s.io/api/batch/v1"

	dockerTypes "github.com/docker/docker/api/types"
	docker "github.com/docker/docker/client"
	"github.com/google/uuid"
	"github.com/onsi/ginkgo/v2"
	"github.com/onsi/ginkgo/v2/dsl/core"
	. "github.com/onsi/gomega"
	"github.com/securesign/operator/api/v1alpha1"
	"gopkg.in/yaml.v2"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/json"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const fromImage = "alpine:latest"

func IsCIEnvironment() bool {
	if val, present := os.LookupEnv("CI"); present {
		b, _ := strconv.ParseBool(val)
		return b
	}
	return false
}

func CreateTestNamespace(ctx context.Context, cli client.Client) *v1.Namespace {
	sp := ginkgo.CurrentSpecReport()
	fn := filepath.Base(sp.LeafNodeLocation.FileName)
	// Replace invalid characters with '-'
	re := regexp.MustCompile("[^a-z0-9-]")
	name := re.ReplaceAllString(strings.TrimSuffix(fn, filepath.Ext(fn)), "-")

	ns := &v1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: name + "-",
		},
	}
	Expect(cli.Create(ctx, ns)).To(Succeed())
	core.GinkgoWriter.Println("Created test namespace: " + ns.Name)
	return ns
}

func PrepareImage(ctx context.Context) string {
	if v, ok := os.LookupEnv("TEST_IMAGE"); ok {
		return v
	}
	targetImageName := fmt.Sprintf("ttl.sh/%s:15m", uuid.New().String())

	dockerCli, err := docker.NewClientWithOpts(docker.FromEnv, docker.WithAPIVersionNegotiation())
	Expect(err).ToNot(HaveOccurred())

	var pull io.ReadCloser
	pull, err = dockerCli.ImagePull(ctx, fromImage, dockerTypes.ImagePullOptions{})
	Expect(err).ToNot(HaveOccurred())
	_, err = io.Copy(core.GinkgoWriter, pull)
	Expect(err).ToNot(HaveOccurred())
	defer func() { _ = pull.Close() }()

	Expect(dockerCli.ImageTag(ctx, fromImage, targetImageName)).To(Succeed())
	var push io.ReadCloser
	push, err = dockerCli.ImagePush(ctx, targetImageName, dockerTypes.ImagePushOptions{RegistryAuth: dockerTypes.RegistryAuthFromSpec})
	Expect(err).ToNot(HaveOccurred())
	_, err = io.Copy(core.GinkgoWriter, push)
	Expect(err).ToNot(HaveOccurred())
	defer func() { _ = push.Close() }()
	// wait for a while to be sure that the image landed in the registry
	time.Sleep(10 * time.Second)
	return targetImageName
}

func EnvOrDefault(env string, def string) string {
	val, ok := os.LookupEnv(env)
	if ok {
		return val
	}
	return def
}

func DumpNamespace(ctx context.Context, cli client.Client, ns string) {

	// Example usage with mock data
	k8s := map[string]logTarget{}

	toDump := map[string]client.ObjectList{
		"securesign.yaml":  &v1alpha1.SecuresignList{},
		"fulcio.yaml":      &v1alpha1.FulcioList{},
		"rekor.yaml":       &v1alpha1.RekorList{},
		"tuf.yaml":         &v1alpha1.TufList{},
		"ctlog.yaml":       &v1alpha1.CTlogList{},
		"trillian.yaml":    &v1alpha1.TrillianList{},
		"pod.yaml":         &v1.PodList{},
		"configmap.yaml":   &v1.ConfigMapList{},
		"deployment.yaml":  &v12.DeploymentList{},
		"replica_set.yaml": &v12.ReplicaSetList{},
		"job.yaml":         &v13.JobList{},
		"cronjob.yaml":     &v13.CronJobList{},
		"event.yaml":       &v1.EventList{},
	}

	core.GinkgoWriter.Println("----------------------- Dumping namespace " + ns + " -----------------------")

	for key, obj := range toDump {
		if dump, err := dumpK8sObjects(ctx, cli, obj, ns); err == nil {
			k8s[key] = logTarget{
				reader: strings.NewReader(dump),
				size:   int64(len(dump)),
			}
		} else {
			log.Println(fmt.Errorf("dump failed for %s: %w", key, err))
		}
	}

	// Create the output file
	fileName := "k8s-dump-" + ns + ".tar.gz"
	outFile, err := os.Create(fileName)
	if err != nil {
		log.Fatalf("Failed to create %s file: %v", fileName, err)
	}

	if err := createArchive(outFile, k8s); err != nil {
		log.Fatalf("Failed to create %s: %v", fileName, err)
	}
}

func dumpK8sObjects(ctx context.Context, cli client.Client, list client.ObjectList, namespace string) (string, error) {
	var builder strings.Builder

	if err := cli.List(ctx, list, client.InNamespace(namespace)); err != nil {
		return "", err
	}

	// Use reflection to access the Items field
	items := reflect.ValueOf(list).Elem().FieldByName("Items")

	// Check if Items field is valid and is a slice
	if !items.IsValid() || items.Kind() != reflect.Slice {
		return "", fmt.Errorf("invalid items field in list: %v", items)
	}

	// Iterate over the items slice
	for i := 0; i < items.Len(); i++ {
		item := items.Index(i).Addr().Interface().(client.Object)
		yamlData := toYAMLNoManagedFields(item)
		builder.WriteString("\n---\n")
		builder.WriteString(yamlData)
	}
	return builder.String(), nil
}

func toYAMLNoManagedFields(value runtime.Object) string {
	object, _ := json.Marshal(value)

	mapdata := map[string]interface{}{}
	_ = json.Unmarshal(object, &mapdata)

	if m, ok := mapdata["metadata"].(map[string]interface{}); ok {
		delete(m, "managedFields")
	}
	out, _ := yaml.Marshal(mapdata)

	return fmt.Sprintf("%s\n", out)
}
