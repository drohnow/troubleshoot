package collect

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"log"
	"path/filepath"
	"strconv"

	troubleshootv1beta2 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"

	"helm.sh/helm/v3/pkg/action"
	"helm.sh/helm/v3/pkg/kube"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/util/homedir"
)

type CollectHelm struct {
	Collector    *troubleshootv1beta2.Helm
	BundlePath   string
	Namespace    string
	ClientConfig *rest.Config
	Client       kubernetes.Interface
	Context      context.Context
	RBACErrors
}

// Helm release information struct
type ReleaseInfo struct {
	ReleaseName string        `json:"releaseName"`
	Chart       string        `json:"chart,omitempty"`
	AppVersion  string        `json:"appVersion,omitempty"`
	Namespace   string        `json:"namespace,omitempty"`
	VersionInfo []VersionInfo `json:"releaseHistory,omitempty"`
}

// Helm release version information struct
type VersionInfo struct {
	Revision  string `json:"revision"`
	Date      string `json:"date"`
	Status    string `json:"status"`
	IsPending bool   `json:"isPending,omitempty"`
}

func (c *CollectHelm) Title() string {
	return getCollectorName(c)
}

func (c *CollectHelm) IsExcluded() (bool, error) {
	return isExcluded(c.Collector.Exclude)
}

func (c *CollectHelm) Collect(progressChan chan<- interface{}) (CollectorResult, error) {
	output := NewResult()

	var kubeconfig *string
	if home := homedir.HomeDir(); home != "" {
		kubeconfig = flag.String("kubeconfig", filepath.Join(home, ".kube", "config"), "(optional) absolute path to the kubeconfig file")
	} else {
		kubeconfig = flag.String("kubeconfig", "", "absolute path to the kubeconfig file")
	}
	flag.Parse()

	//releaseName := "prod01"

	releaseInfo := helmReleaseHistoryCollector(c.Collector.ReleaseName, kubeconfig)
	log.Println(releaseInfo)

	helmHistoryJson, errJson := json.MarshalIndent(releaseInfo, "", "\t")
	if errJson != nil {
		log.Println(errJson)
	}

	log.Println(string(helmHistoryJson))

	filePath := "helm/helm.json"

	err := output.SaveResult(c.BundlePath, filePath, bytes.NewBuffer(helmHistoryJson))
	if err != nil {
		return nil, err
	}

	return output, nil

}

func helmReleaseHistoryCollector(releaseName string, kubeconfig *string) ReleaseInfo {
	actionConfig := new(action.Configuration)
	err := actionConfig.Init(kube.GetConfig(*kubeconfig, "", ""), "", "", log.Printf)
	if err != nil {
		log.Fatal(err)
	}

	var results ReleaseInfo

	releases, err := action.NewHistory(actionConfig).Run(releaseName)
	if err != nil {
		log.Fatal(err)
	}

	for _, rel := range releases {
		//log.Println(release.Name)
		actionConfig := new(action.Configuration)
		err := actionConfig.Init(kube.GetConfig(*kubeconfig, "", ""), "", "", log.Printf)
		if err != nil {
			log.Fatal(err)
		}

		//	release, _ := action.NewList(actionConfig).Run()

		versionInfo := getVersionInfo(rel.Name, kubeconfig)

		results = ReleaseInfo{
			ReleaseName: rel.Name,
			Chart:       rel.Chart.Name(),
			AppVersion:  rel.Chart.AppVersion(),
			Namespace:   rel.Namespace,
			VersionInfo: versionInfo,
		}

		//slog.Println("mycollection: ", collection)

	}
	return results
}

func getVersionInfo(releaseName string, kubeconfig *string) []VersionInfo {

	actionConfig := new(action.Configuration)
	err := actionConfig.Init(kube.GetConfig(*kubeconfig, "", ""), "", "", log.Printf)
	if err != nil {
		log.Fatal(err)
	}

	versionCollect := []VersionInfo{}

	history, _ := action.NewHistory(actionConfig).Run(releaseName)

	for _, rel := range history {

		versionCollect = append(versionCollect, VersionInfo{
			Revision:  strconv.Itoa(rel.Version),
			Date:      rel.Info.LastDeployed.String(),
			Status:    rel.Info.Status.String(),
			IsPending: rel.Info.Status.IsPending(),
		})
	}
	return versionCollect
}
