/*
Copyright 2021.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controllers

import (
	"context"

	"strconv" //型変換用

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	nginxcontrollerv1 "my.domain/nginx-controller/api/v1"

	appsv1 "k8s.io/api/apps/v1"                   // apps/v1グループのAPIを扱う
	corev1 "k8s.io/api/core/v1"                   // coreグループのAPIを扱う
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1" // metafieldを扱う
	"k8s.io/client-go/tools/record"               // k8s eventリソースに対するRecorderを使うため
)

// NginxReconciler reconciles a Nginx object
type NginxReconciler struct {
	client.Client
	Scheme   *runtime.Scheme
	Log      logr.Logger
	Recorder record.EventRecorder
}

//+kubebuilder:rbac:groups=nginxcontroller.my.domain,resources=nginxes,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=nginxcontroller.my.domain,resources=nginxes/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=nginxcontroller.my.domain,resources=nginxes/finalizers,verbs=update
//+kubebuilder:rbac:groups=apps,resources=deployments,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups="",resources=events,verbs=create;patch

func (r *NginxReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	Logger := log.FromContext(ctx)

	Logger.Info("★Reconcile Function Called★")

	// nginxオブジェクトの取得
	Logger.Info("[Debug] req(NamespacedName):" + req.Namespace + "/" + req.Name)
	Logger.Info("[Step1] fetching Nginx Resource")
	var nginx nginxcontrollerv1.Nginx
	if err := r.Get(ctx, req.NamespacedName, &nginx); err != nil {
		Logger.Error(err, "Unable to get Nginx from in-memory-cache")
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}
	Logger.Info("[Debug] Nginx: " + nginx.Namespace + "/" + nginx.Name + "from in-memory-cache")

	// nginxが過去に管理していた古いdeploymentが存在すれば削除する
	Logger.Info("[Step2]Delete old Deployment")
	if err := r.cleanupOwnedResources(ctx, Logger, &nginx); err != nil {
		Logger.Error(err, "Faild to clean up old Deployment resource for this Nginx")
		return ctrl.Result{}, err
	}

	// nginxが管理するDeploymentが存在しなければ作成・更新する
	Logger.Info("[Step3]Get Deployment info from nginx")
	deploymentName := nginx.Spec.DeploymentName // nginxオブジェクトが管理するDeploymentの名前を取得
	deploy := &appsv1.Deployment{               // Deploymentテンプレートを作成
		ObjectMeta: metav1.ObjectMeta{ // ObjectMetaフィールドの定義
			Name:      deploymentName,
			Namespace: req.Namespace,
		},
	}
	//[Debug]対象のDeployment情報出力
	Logger.Info("[Debug]Managed Deployment info: " + deploy.Namespace + "/" + deploy.Name)

	// func(MutateFn)で用意したdeployが無かったら作成、更新が必要であれば更新
	Logger.Info("[Debug]CreateOrUpdate Deployment: " + deploy.Namespace + "/" + deploy.Name)
	if _, err := ctrl.CreateOrUpdate(ctx, r.Client, deploy, func() error {
		// deployのreplicasにnginxで定義した値を入れる
		replicas := int32(1)
		if nginx.Spec.Replicas != nil {
			replicas = *nginx.Spec.Replicas
		}
		deploy.Spec.Replicas = &replicas

		// deployのlabelsに任意の値を入れる
		labels := map[string]string{
			"app":        "nginx",
			"controller": req.Name,
		}
		if deploy.Spec.Selector == nil {
			deploy.Spec.Selector = &metav1.LabelSelector{MatchLabels: labels}
		}

		if deploy.Spec.Template.ObjectMeta.Labels == nil {
			deploy.Spec.Template.ObjectMeta.Labels = labels
		}

		// deployのcontainersに任意の値を入れる
		containers := []corev1.Container{
			{
				Name:  "nginx",
				Image: "nginx:latest",
			},
		}
		if deploy.Spec.Template.Spec.Containers == nil {
			deploy.Spec.Template.Spec.Containers = containers
		}

		// ★deployのOwnerReferenceにNginxオブジェクトを設定する
		// これをすることでこのDeploymentの親オブジェクトがfooであることを示し、
		// fooオブジェクトが削除されたらGCによりdeploymentも削除されるようになる
		if err := ctrl.SetControllerReference(&nginx, deploy, r.Scheme); err != nil {
			Logger.Error(err, "Unable to set ownerReference from Nginx to Deployment")
			return err
		}

		// ここまでの処理でNginxオブジェクトに紐づくDeploymentテンプレート（deployが出来上がる）
		// これをctrl.CreateOrUpdateにつっこむ

		return nil
	}); err != nil {
		Logger.Error(err, "Unable to ensure Deployment is correct")
		return ctrl.Result{}, err
	}

	// nginxオブジェクトのステータスを更新
	Logger.Info("[Step4] Update Nginx status")
	var deployment appsv1.Deployment
	// in-memory-cacheからとってきた情報と同じnamespace/nameの構造体を作成
	var deploymentNamespacedName = client.ObjectKey{
		Namespace: req.Namespace,
		Name:      nginx.Spec.DeploymentName,
	}

	if err := r.Get(ctx, deploymentNamespacedName, &deployment); err != nil {
		Logger.Error(err, "Unable to get Deployment from in-memory-cache")
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	availableReplicas := deployment.Status.AvailableReplicas
	deploymentStatus := strconv.Itoa(int(deployment.Status.ReadyReplicas)) + "/" + strconv.Itoa(int(deployment.Status.Replicas))

	// statusが一致したらreconcile終了
	if nginx.Status.DeploymentStatus == deploymentStatus && nginx.Status.AvailableReplicas == availableReplicas {
		return ctrl.Result{}, nil
	}

	// statusが一致しない場合、更新する
	Logger.Info("[Debug] Update Nginx status nginx.Status.DeploymentStatus")
	nginx.Status.DeploymentStatus = deploymentStatus
	nginx.Status.AvailableReplicas = availableReplicas
	if err := r.Status().Update(ctx, &nginx); err != nil {
		Logger.Error(err, "Unable to update Nginx status nginx.Status.DeploymentStatus")
		return ctrl.Result{}, err
	}
	r.Recorder.Eventf(&nginx, corev1.EventTypeNormal, "Update", "Update nginx.Status.DeploymentStatus: %d", nginx.Status.DeploymentStatus)
	r.Recorder.Eventf(&nginx, corev1.EventTypeNormal, "Update", "Update nginx.status.AvailableReplicas: %d", nginx.Status.AvailableReplicas)

	return ctrl.Result{}, nil
}

func (r *NginxReconciler) cleanupOwnedResources(ctx context.Context, log logr.Logger, nginx *nginxcontrollerv1.Nginx) error {
	log.Info("Finding existing Deployments of nginx resource")

	// Nginxオブジェクトが管理するDeploymentをIn-Memory-CacheからListerで取得
	var deployments appsv1.DeploymentList
	if err := r.List(ctx, &deployments, client.InNamespace(nginx.Namespace), client.MatchingFields(map[string]string{deploymentOwnerKey: nginx.Name})); err != nil {
		return err
	}
	// 取得したDeploymentの削除処理
	for _, deployment := range deployments.Items {
		if deployment.Name == nginx.Spec.DeploymentName { // deployment名がnginxオブジェクトで管理するものと一致するかチェック
			continue // 一致するなら後ろの処理はスキップ
		}
		// deploymentを削除
		if err := r.Delete(ctx, &deployment); err != nil {
			log.Error(err, "Faild to delete old Deployment resource")
			return err
		}
		// ログ出力 & イベント記録
		log.Info("Delete Deployment resource: " + deployment.Name)
		r.Recorder.Eventf(nginx, corev1.EventTypeNormal, "Deleted", "Deleted Deployment %q", deployment.Name)
	}
	return nil
}

var (
	deploymentOwnerKey = ".metadata.controller"
	apiGVStr           = nginxcontrollerv1.GroupVersion.String()
)

// SetupWithManager sets up the controller with the Manager.
func (r *NginxReconciler) SetupWithManager(mgr ctrl.Manager) error {

	// DeploymentにdeploymentOwnerKeyを追加する
	// 実行にあたり引数になっているfunc(MutateFn)を先に実行する
	if err := mgr.GetFieldIndexer().IndexField(context.Background(), &appsv1.Deployment{}, deploymentOwnerKey, func(rawObj client.Object) []string {
		deployment := rawObj.(*appsv1.Deployment)
		owner := metav1.GetControllerOf(deployment)
		if owner == nil {
			return nil
		}
		if owner.APIVersion != apiGVStr || owner.Kind != "Nginx" {
			return nil
		}
		return []string{owner.Name}
	}); err != nil {
		return err
	}

	return ctrl.NewControllerManagedBy(mgr).
		For(&nginxcontrollerv1.Nginx{}). // Controllerが管理するリソース
		Owns(&appsv1.Deployment{}).      // Custom Resourceの子に当たるリソース
		Complete(r)
}
