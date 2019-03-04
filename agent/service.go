package agent

import (
	"github.com/containerd/containerd"
	containers "github.com/containerd/containerd/api/services/containers/v1"
	diff "github.com/containerd/containerd/api/services/diff/v1"
	images "github.com/containerd/containerd/api/services/images/v1"
	namespaces "github.com/containerd/containerd/api/services/namespaces/v1"
	tasks "github.com/containerd/containerd/api/services/tasks/v1"
	"github.com/containerd/containerd/content"
	"github.com/containerd/containerd/leases"
	"github.com/containerd/containerd/plugin"
	"github.com/containerd/containerd/services"
	"github.com/containerd/containerd/snapshots"
	v1 "github.com/crosbymichael/boss/api/v1"
	"github.com/crosbymichael/boss/config"
	"github.com/pkg/errors"
	"google.golang.org/grpc"
)

func init() {
	c, err := config.Load()
	if err != nil {
		panic(err)
	}
	plugin.Register(&plugin.Registration{
		Type:   plugin.GRPCPlugin,
		ID:     "boss",
		Config: c,
		Requires: []plugin.Type{
			plugin.ServicePlugin,
		},
		InitFn: func(ic *plugin.InitContext) (interface{}, error) {
			// ic.Meta.Platforms = []imagespec.Platform{platforms.DefaultSpec()}
			//	ic.Meta.Exports = map[string]string{"CRIVersion": constants.CRIVersion}
			c := ic.Config.(*config.Config)
			if err != nil {
				return nil, err
			}
			store, err := c.Store()
			if err != nil {
				return nil, err
			}
			servicesOpts, err := getServicesOpts(ic)
			if err != nil {
				return nil, err
			}
			client, err := containerd.New(
				"",
				containerd.WithDefaultNamespace(v1.DefaultNamespace),
				containerd.WithServices(servicesOpts...),
			)
			if err != nil {
				return nil, err
			}
			return New(c, client, store)
		},
	})
}

func (a *Agent) Register(server *grpc.Server) error {
	v1.RegisterAgentServer(server, a)
	return nil
}

func (a *Agent) RegisterTCP(server *grpc.Server) error {
	v1.RegisterAgentServer(server, a)
	return nil
}

// getServicesOpts get service options from plugin context.
func getServicesOpts(ic *plugin.InitContext) ([]containerd.ServicesOpt, error) {
	plugins, err := ic.GetByType(plugin.ServicePlugin)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get service plugin")
	}

	opts := []containerd.ServicesOpt{
		containerd.WithEventService(ic.Events),
	}
	for s, fn := range map[string]func(interface{}) containerd.ServicesOpt{
		services.ContentService: func(s interface{}) containerd.ServicesOpt {
			return containerd.WithContentStore(s.(content.Store))
		},
		services.ImagesService: func(s interface{}) containerd.ServicesOpt {
			return containerd.WithImageService(s.(images.ImagesClient))
		},
		services.SnapshotsService: func(s interface{}) containerd.ServicesOpt {
			return containerd.WithSnapshotters(s.(map[string]snapshots.Snapshotter))
		},
		services.ContainersService: func(s interface{}) containerd.ServicesOpt {
			return containerd.WithContainerService(s.(containers.ContainersClient))
		},
		services.TasksService: func(s interface{}) containerd.ServicesOpt {
			return containerd.WithTaskService(s.(tasks.TasksClient))
		},
		services.DiffService: func(s interface{}) containerd.ServicesOpt {
			return containerd.WithDiffService(s.(diff.DiffClient))
		},
		services.NamespacesService: func(s interface{}) containerd.ServicesOpt {
			return containerd.WithNamespaceService(s.(namespaces.NamespacesClient))
		},
		services.LeasesService: func(s interface{}) containerd.ServicesOpt {
			return containerd.WithLeasesService(s.(leases.Manager))
		},
	} {
		p := plugins[s]
		if p == nil {
			return nil, errors.Errorf("service %q not found", s)
		}
		i, err := p.Instance()
		if err != nil {
			return nil, errors.Wrapf(err, "failed to get instance of service %q", s)
		}
		if i == nil {
			return nil, errors.Errorf("instance of service %q not found", s)
		}
		opts = append(opts, fn(i))
	}
	return opts, nil
}
