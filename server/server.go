package server

import (
	"fmt"
	"net/http"

	log "github.com/Sirupsen/logrus"
	"github.com/docker/machine/store"
	"github.com/gorilla/mux"
)

type Server struct {
	listenAddress      string
	sslCertificatePath string
	sslKeyPath         string
	store              *store.Store
}

func NewServer(listenAddress string, sslCertificatePath string, sslKeyPath string, store *store.Store) (*Server, error) {
	if sslCertificatePath == "" || sslKeyPath == "" {
		return nil, fmt.Errorf("ssl certificate and key path must be specified")
	}

	srv := &Server{
		listenAddress:      listenAddress,
		sslCertificatePath: sslCertificatePath,
		sslKeyPath:         sslKeyPath,
		store:              store,
	}

	return srv, nil
}

func (s *Server) Run() error {
	r := mux.NewRouter()
	r.HandleFunc("/api/machines", s.getMachines).Methods("GET")

	log.Infof("Machine server listening on %s", s.listenAddress)
	return http.ListenAndServeTLS(s.listenAddress, s.sslCertificatePath, s.sslKeyPath, nil)
}

func (s *Server) getMachines(w http.ResponseWriter, r *http.Request) {
	w.Write([]byte("getMachines"))
}
