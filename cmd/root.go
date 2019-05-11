// Copyright © 2019 NAME HERE <EMAIL ADDRESS>
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package cmd

import (
	"fmt"
	"os"
	"time"

	"github.com/golang/glog"
	homedir "github.com/mitchellh/go-homedir"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/informers"
	coreinformers "k8s.io/client-go/informers/core/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/clientcmd"
)

var cfgFile string
var clientset *kubernetes.Clientset
var kubeconfig string

// RootCmd represents the base command when called without any subcommands
var RootCmd = &cobra.Command{
	Use:   "operator",
	Short: "A brief description of your application",
	Long: `A longer description that spans multiple lines and likely contains
examples and usage of using your application. For example:

Cobra is a CLI library for Go that empowers applications.
This application is a tool to generate the needed files
to quickly create a Cobra application.`,
	// Uncomment the following line if your bare application
	// has an action associated with it:
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("my operator")
		// use the current context in kubeconfig
		config, err := clientcmd.BuildConfigFromFlags("", kubeconfig)
		if err != nil {
			panic(err.Error())
		}

		// create the clientset
		clientset, err := kubernetes.NewForConfig(config)
		if err != nil {
			panic(err.Error())
		}
		// 创建一个factory
		sharedInformerFactory := informers.NewSharedInformerFactoryWithOptions(clientset, time.Minute*1)
		// start()
		controller := NewPodLoggingController(sharedInformerFactory)
		stop := make(chan struct{})
		defer close(stop)
		err = controller.Run(stop)
		if err != nil {
			glog.Fatal(err)
		}
		select {}
		for {
			pods, err := clientset.CoreV1().Pods("").List(metav1.ListOptions{
				ResourceVersion: "0",
			})
			if err != nil {
				panic(err.Error())
			}
			fmt.Printf("There are %d pods in the cluster\n", len(pods.Items))

			// Examples for error handling:
			// - Use helper functions like e.g. errors.IsNotFound()
			// - And/or cast to StatusError and use its properties like e.g. ErrStatus.Message
			namespace := "default"
			pod := "main001-000"
			_, err = clientset.CoreV1().Pods(namespace).Get(pod, metav1.GetOptions{})
			if errors.IsNotFound(err) {
				fmt.Printf("Pod %s in namespace %s not found\n", pod, namespace)
			} else if statusError, isStatus := err.(*errors.StatusError); isStatus {
				fmt.Printf("Error getting pod %s in namespace %s: %v\n",
					pod, namespace, statusError.ErrStatus.Message)
			} else if err != nil {
				panic(err.Error())
			} else {
				fmt.Printf("Found pod %s in namespace %s\n", pod, namespace)
			}

			time.Sleep(1 * time.Second)
		}
	},
}

func homeDir() string {
	if h := os.Getenv("HOME"); h != "" {
		return h
	}
	return os.Getenv("USERPROFILE") // windows
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	if err := RootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

func init() {
	cobra.OnInitialize(initConfig)

	// Here you will define your flags and configuration settings.
	// Cobra supports persistent flags, which, if defined here,
	// will be global for your application.
	RootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is $HOME/.operator.yaml)")
	RootCmd.PersistentFlags().StringVar(&kubeconfig, "kubeconfig", "", "absolute path to the kubeconfig file")
	// Cobra also supports local flags, which will only run
	// when this action is called directly.
	RootCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
}

// initConfig reads in config file and ENV variables if set.
func initConfig() {
	if cfgFile != "" {
		// Use config file from the flag.
		viper.SetConfigFile(cfgFile)
	} else {
		// Find home directory.
		home, err := homedir.Dir()
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}

		// Search config in home directory with name ".operator" (without extension).
		viper.AddConfigPath(home)
		viper.SetConfigName(".operator")
	}

	viper.AutomaticEnv() // read in environment variables that match

	// If a config file is found, read it in.
	if err := viper.ReadInConfig(); err == nil {
		fmt.Println("Using config file:", viper.ConfigFileUsed())
	}
}

// PodLoggingController logs the name and namespace of pods that are added,
// deleted, or updated
type PodLoggingController struct {
	informerFactory informers.SharedInformerFactory
	podInformer     coreinformers.PodInformer
}

// Run starts shared informers and waits for the shared informer cache to
// synchronize.
func (c *PodLoggingController) Run(stopCh chan struct{}) error {
	// Starts all the shared informers that have been created by the factory so
	// far.
	c.informerFactory.Start(stopCh)
	// wait for the initial synchronization of the local cache.
	if !cache.WaitForCacheSync(stopCh, c.podInformer.Informer().HasSynced) {
		return fmt.Errorf("Failed to sync")
	}
	return nil
}
func (c *PodLoggingController) podAdd(obj interface{}) {
	pod := obj.(*v1.Pod)
	glog.Infof("POD CREATED: %s/%s", pod.Namespace, pod.Name)
}

func (c *PodLoggingController) podUpdate(old, new interface{}) {
	oldPod := old.(*v1.Pod)
	newPod := new.(*v1.Pod)
	glog.Infof(
		"POD UPDATED. %s/%s %s",
		oldPod.Namespace, oldPod.Name, newPod.Status.Phase,
	)
}

func (c *PodLoggingController) podDelete(obj interface{}) {
	pod := obj.(*v1.Pod)
	glog.Infof("POD DELETED: %s/%s", pod.Namespace, pod.Name)
}

// NewPodLoggingController creates a PodLoggingController
func NewPodLoggingController(informerFactory informers.SharedInformerFactory) *PodLoggingController {
	podInformer := informerFactory.Core().V1().Pods()
	lister := podInformer.Lister()
	go func() {
		for {
			time.Sleep(time.Second * 10)
			glog.Info(podInformer.Informer().HasSynced())
			pods, err := lister.List(labels.Everything())
			if err != nil {
				panic(err)
			}
			for _, p := range pods {
				glog.Infof("Pod: [%s]", p.Name)
			}
		}
	}()

	c := &PodLoggingController{
		informerFactory: informerFactory,
		podInformer:     podInformer,
	}
	podInformer.Informer().AddEventHandler(
		// Your custom resource event handlers.
		cache.ResourceEventHandlerFuncs{
			// Called on creation
			AddFunc: c.podAdd,
			// Called on resource update and every resyncPeriod on existing resources.
			UpdateFunc: c.podUpdate,
			// Called on resource deletion.
			DeleteFunc: c.podDelete,
		},
	)
	return c
}
