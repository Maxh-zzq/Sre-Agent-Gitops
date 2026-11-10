package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"

	v1 "k8s.io/api/core/v1"
	"k8s.io/client-go/rest" // 确保导入
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// --- 配置常量 ---
const (
	prometheusURL = "http://192.168.30.11:30090" // 替换成你的Prometheus NodePort IP和端口
	ollamaURL     = "http://192.168.30.11:31312" // 替换成你的Ollama NodePort IP和端口 
	ollamaModel   = "qwen2.5:1.5b"
)

// --- 数据结构定义 ---

// Prometheus告警响应结构体
type PrometheusAlerts struct {
	Status string `json:"status"`
	Data   struct {
		Alerts []Alert `json:"alerts"`
	} `json:"data"`
}

// 单个告警的结构体
type Alert struct {
	Labels      map[string]string `json:"labels"`
	Annotations map[string]string `json:"annotations"`
	State       string            `json:"state"`
	ActiveAt    time.Time         `json:"activeAt"`
}

// Ollama请求结构体
type OllamaRequest struct {
	Model  string `json:"model"`
	Prompt string `json:"prompt"`
	Stream bool   `json:"stream"`
}

// Ollama响应结构体
type OllamaResponse struct {
	Response string `json:"response"`
}

// --- 主函数 ---
func main() {
	log.Println("Starting K8s SRE Agent...")

	// 初始化Kubernetes客户端
	clientset, err := getKubernetesClient()
	if err != nil {
		log.Fatalf("Error creating Kubernetes client: %v", err)
	}

	// 启动智能体的主循环
	agentLoop(clientset)
}

// 智能体主循环
func agentLoop(clientset *kubernetes.Clientset) {
	for {
		log.Println("--- Agent Loop Start ---")

		// 1. 感知：查询Prometheus告警
		alerts, err := queryPrometheusAlerts()
		if err != nil {
			log.Printf("Error querying Prometheus: %v", err)
			time.Sleep(1 * time.Minute) // 如果查询失败，等待更长时间
			continue
		}

		if len(alerts) == 0 {
			log.Println("No active alerts. System is healthy.")
		} else {
			log.Printf("Found %d active alert(s).", len(alerts))
			// 2. 思考与行动：处理每个告警
			for _, alert := range alerts {
				if alert.Labels["alertname"] == "PodCrashLooping" {
					handlePodCrashLooping(clientset, alert)
				}
			}
		}

		log.Println("--- Agent Loop End ---")
		time.Sleep(30 * time.Second) // 每30秒检查一次
	}
}

// 感知 (Perception) 
func queryPrometheusAlerts() ([]Alert, error) {
	resp, err := http.Get(prometheusURL + "/api/v1/alerts")
	if err != nil {
		return nil, fmt.Errorf("failed to get alerts from Prometheus: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("prometheus returned non-200 status code: %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	var alertResponse PrometheusAlerts
	if err := json.Unmarshal(body, &alertResponse); err != nil {
		return nil, fmt.Errorf("failed to unmarshal json: %w", err)
	}

	var firingAlerts []Alert
	for _, alert := range alertResponse.Data.Alerts {
		if alert.State == "firing" {
			firingAlerts = append(firingAlerts, alert)
		}
	}
	return firingAlerts, nil
}

func handlePodCrashLooping(clientset *kubernetes.Clientset, alert Alert) {
    podName := alert.Labels["pod"]
    namespace := alert.Labels["namespace"]
    reason := alert.Annotations["description"] // 从告警的描述中获取原因
    log.Printf("Handling PodCrashLooping alert for %s/%s", namespace, podName)

    // 不再需要收集详细上下文，直接请求LLM决策
    decision, err := getLLMDecision(namespace, podName, reason)
    if err != nil {
        log.Printf("Failed to get decision from LLM: %v", err)
        return
    }
    log.Printf("LLM Decision: %s", decision)

    // 执行行动
    if err := executeAction(clientset, namespace, podName, decision); err != nil {
        log.Printf("Failed to execute action '%s' for pod %s/%s: %v", decision, namespace, podName, err)
    }
}

// 收集Pod的详细信息
func getPodContext(clientset *kubernetes.Clientset, namespace, podName string) (string, error) {
	var contextBuilder strings.Builder

	// 获取Pod对象	
	pod, err := clientset.CoreV1().Pods(namespace).Get(context.TODO(), podName, metav1.GetOptions{})
	if err != nil {
		return "", err
	}
	contextBuilder.WriteString(fmt.Sprintf("Pod Status: %s\n", pod.Status.Phase))
	if len(pod.Status.ContainerStatuses) > 0 {
		status := pod.Status.ContainerStatuses[0]
		if status.State.Waiting != nil {
			contextBuilder.WriteString(fmt.Sprintf("Container Waiting Reason: %s\n", status.State.Waiting.Reason))
		}
	}

	// 获取Pod日志
	podLogOpts := v1.PodLogOptions{TailLines: new(int64)}
	*podLogOpts.TailLines = 50 // 获取最后50行日志
	req := clientset.CoreV1().Pods(namespace).GetLogs(podName, &podLogOpts)
	podLogs, err := req.Stream(context.TODO())
	if err != nil {
		return "", fmt.Errorf("error in opening stream: %w", err)
	}
	defer podLogs.Close()
	buf := new(bytes.Buffer)
	_, err = io.Copy(buf, podLogs)
	if err != nil {
		return "", fmt.Errorf("error in copy information from podLogs to buf: %w", err)
	}
	contextBuilder.WriteString(fmt.Sprintf("\n--- Pod Logs ---\n%s\n", buf.String()))

	// 获取Pod事件
	events, err := clientset.CoreV1().Events(namespace).List(context.TODO(), metav1.ListOptions{
		FieldSelector: "involvedObject.name=" + podName,
	})
	if err != nil {
		return "", err
	}
	contextBuilder.WriteString("\n--- Pod Events ---\n")
	for _, event := range events.Items {
		contextBuilder.WriteString(fmt.Sprintf("- %s: %s\n", event.Reason, event.Message))
	}

	return contextBuilder.String(), nil
}

// 请求LLM进行分析和决策
func getLLMDecision(namespace, podName, reason string) (string, error) {
    prompt := fmt.Sprintf(
        `A pod is in a CrashLoopBackOff state.
        - Namespace: %s
        - Pod: %s
        - Reason: %s
        Based on this, should I restart the pod? Your response should contain the word "restart" if you recommend restarting.`, namespace, podName, reason)

    reqBody, err := json.Marshal(OllamaRequest{
        Model:  ollamaModel,
        Prompt: prompt,
        Stream: false,
    })
    if err != nil {
        return "", err
    }

    resp, err := http.Post(ollamaURL+"/api/generate", "application/json", bytes.NewBuffer(reqBody))
    if err != nil {
        return "", err
    }
    defer resp.Body.Close()

    body, err := io.ReadAll(resp.Body)
    if err != nil {
        return "", err
    }

    var ollamaResp OllamaResponse
    if err := json.Unmarshal(body, &ollamaResp); err != nil {
        log.Printf("DEBUG: Failed to unmarshal LLM response body: %s", string(body))
        return "", err
    }

    responseText := strings.ToLower(ollamaResp.Response) // 转换为小写
    log.Printf("DEBUG: Raw LLM response: '%s'", ollamaResp.Response)

    if strings.Contains(responseText, "restart") {
        log.Println("LLM response contains 'restart'. Interpreting as RESTART action.")
        return "RESTART", nil
    }
    
    log.Println("LLM response does not contain 'restart'. Interpreting as IGNORE action.")
    return "IGNORE", nil
    // ---------------------
}

// 执行恢复动作
func executeAction(clientset *kubernetes.Clientset, namespace, podName, action string) error {
	log.Printf("Executing action: %s on pod %s/%s", action, namespace, podName)
	switch action {
	case "RESTART":
		// 在K8s中，重启一个由Deployment管理的Pod最简单的方法是删除它
		// Deployment Controller会自动创建一个新的Pod来替代
		log.Printf("Deleting pod %s/%s to trigger restart...", namespace, podName)
		return clientset.CoreV1().Pods(namespace).Delete(context.TODO(), podName, metav1.DeleteOptions{})
	case "IGNORE":
		log.Println("Action is IGNORE. No operation will be performed.")
		return nil
	default:
		return fmt.Errorf("unknown action: %s", action)
	}
}

// 获取K8s客户端
func getKubernetesClient() (*kubernetes.Clientset, error) {
    config, err := rest.InClusterConfig() // 优先尝试In-Cluster配置
    if err != nil {
        log.Printf("Failed to get in-cluster config: %v. Falling back to kubeconfig for local dev.", err)
        // 回退到使用kubeconfig文件
        homeDir := os.Getenv("HOME")
        if homeDir == "" {
             return nil, fmt.Errorf("HOME environment variable not set")
        }
        kubeconfig := filepath.Join(homeDir, ".kube", "config")
        config, err = clientcmd.BuildConfigFromFlags("", kubeconfig)
        if err != nil {
            return nil, fmt.Errorf("error building kubeconfig: %w", err)
        }
    }

    clientset, err := kubernetes.NewForConfig(config)
    if err != nil {
        return nil, fmt.Errorf("error creating clientset: %w", err)
    }
    return clientset, nil
}
