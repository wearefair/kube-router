package watchers

import (
	"reflect"
	"time"

	"github.com/cloudnativelabs/kube-router/utils"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/client-go/kubernetes"
	api "k8s.io/client-go/pkg/api/v1"
	cache "k8s.io/client-go/tools/cache"
)

type ServiceUpdate struct {
	Service *api.Service
	Op      Operation
}

var (
	ServiceWatcher *serviceWatcher
)

type serviceWatcher struct {
	clientset         *kubernetes.Clientset
	serviceController cache.Controller
	serviceLister     cache.Indexer
	broadcaster       *utils.Broadcaster
}

type ServiceUpdatesHandler interface {
	OnServiceUpdate(serviceUpdate *ServiceUpdate)
}

func (svcw *serviceWatcher) serviceAddEventHandler(obj interface{}) {
	service, ok := obj.(*api.Service)
	if !ok {
		return
	}
	svcw.broadcaster.Notify(&ServiceUpdate{Op: ADD, Service: service})
}

func (svcw *serviceWatcher) serviceDeleteEventHandler(obj interface{}) {
	service, ok := obj.(*api.Service)
	if !ok {
		return
	}
	svcw.broadcaster.Notify(&ServiceUpdate{Op: REMOVE, Service: service})
}

func (svcw *serviceWatcher) serviceAUpdateEventHandler(oldObj, newObj interface{}) {
	service, ok := newObj.(*api.Service)
	if !ok {
		return
	}
	if !reflect.DeepEqual(newObj, oldObj) {
		svcw.broadcaster.Notify(&ServiceUpdate{Op: UPDATE, Service: service})
	}
}

func (svcw *serviceWatcher) RegisterHandler(handler ServiceUpdatesHandler) {
	svcw.broadcaster.Add(utils.ListenerFunc(func(instance interface{}) {
		handler.OnServiceUpdate(instance.(*ServiceUpdate))
	}))
}

func (svcw *serviceWatcher) List() []*api.Service {
	obj_list := svcw.serviceLister.List()
	svc_instances := make([]*api.Service, len(obj_list))
	for i, ins := range obj_list {
		svc_instances[i] = ins.(*api.Service)
	}
	return svc_instances
}

func (svcw *serviceWatcher) HasSynced() bool {
	return svcw.serviceController.HasSynced()
}

var servicesStopCh chan struct{}

func StartServiceWatcher(clientset *kubernetes.Clientset, resyncPeriod time.Duration) (*serviceWatcher, error) {

	svcw := serviceWatcher{}
	ServiceWatcher = &svcw

	eventHandler := cache.ResourceEventHandlerFuncs{
		AddFunc:    svcw.serviceAddEventHandler,
		DeleteFunc: svcw.serviceDeleteEventHandler,
		UpdateFunc: svcw.serviceAUpdateEventHandler,
	}

	svcw.clientset = clientset
	svcw.broadcaster = utils.NewBroadcaster()
	lw := cache.NewListWatchFromClient(clientset.Core().RESTClient(), "services", metav1.NamespaceAll, fields.Everything())
	svcw.serviceLister, svcw.serviceController = cache.NewIndexerInformer(
		lw,
		&api.Service{}, resyncPeriod, eventHandler,
		cache.Indexers{cache.NamespaceIndex: cache.MetaNamespaceIndexFunc},
	)
	servicesStopCh = make(chan struct{})
	go svcw.serviceController.Run(servicesStopCh)
	return &svcw, nil
}
func StopServiceWatcher() {
	servicesStopCh <- struct{}{}
}
