// Copyright 2020 PingCAP, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// See the License for the specific language governing permissions and
// limitations under the License.

package util

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

	. "github.com/onsi/gomega"
	appconstant "github.com/pingcap/tidb-operator/cmd/backup-manager/app/constants"
	"github.com/pingcap/tidb-operator/pkg/apis/pingcap/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	apiequality "k8s.io/apimachinery/pkg/api/equality"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/pointer"
)

func TestConstructDumplingOptionsForBackup(t *testing.T) {
	g := NewGomegaWithT(t)

	type testcase struct {
		name              string
		hasBackupFilter   bool
		hasDumplingFilter bool
		hasOptions        bool
	}

	tests := []*testcase{
		{
			name:              "backup filter is empty and dumpling config is empty",
			hasOptions:        false,
			hasBackupFilter:   false,
			hasDumplingFilter: false,
		},
		{
			name:              "backup filter is empty and customize dumpling options but not set table regex",
			hasOptions:        true,
			hasBackupFilter:   false,
			hasDumplingFilter: false,
		},
		{
			name:              "backup filter is empty and customize dumpling table regex but not customize options",
			hasOptions:        false,
			hasBackupFilter:   false,
			hasDumplingFilter: true,
		},
		{
			name:              "backup filter is empty and customize dumpling table regex and customize options",
			hasOptions:        true,
			hasBackupFilter:   false,
			hasDumplingFilter: true,
		},
		{
			name:              "customize backup filter and dumpling config is empty",
			hasOptions:        false,
			hasBackupFilter:   true,
			hasDumplingFilter: false,
		},
		{
			name:              "customize backup filter and customize dumpling options but not set table regex",
			hasOptions:        true,
			hasBackupFilter:   true,
			hasDumplingFilter: false,
		},
		{
			name:              "customize backup filter and customize dumpling table regex but not customize options",
			hasOptions:        false,
			hasBackupFilter:   true,
			hasDumplingFilter: true,
		},
		{
			name:              "customize backup filter and customize dumpling table regex and customize options",
			hasOptions:        true,
			hasBackupFilter:   true,
			hasDumplingFilter: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			backup := newBackup()

			customBackupFilter := []string{"mysql.*"}
			customDumplingFilter := []string{"mysql2.*"}
			customOptions := []string{"--consistency=snapshot"}

			var expectArgs []string

			if tt.hasBackupFilter || tt.hasDumplingFilter || tt.hasOptions {
				backup.Spec.Dumpling = &v1alpha1.DumplingConfig{}
			}

			if tt.hasBackupFilter {
				backup.Spec.TableFilter = customBackupFilter
				expectArgs = append(expectArgs, "--filter", customBackupFilter[0])
			} else if tt.hasDumplingFilter {
				backup.Spec.Dumpling.TableFilter = customDumplingFilter
				expectArgs = append(expectArgs, "--filter", customDumplingFilter[0])
			} else {
				expectArgs = append(expectArgs, defaultTableFilterOptions...)
			}

			if tt.hasOptions {
				backup.Spec.Dumpling.Options = customOptions
				expectArgs = append(expectArgs, customOptions...)
			} else {
				expectArgs = append(expectArgs, defaultOptions...)
			}

			generateArgs := ConstructDumplingOptionsForBackup(backup)
			g.Expect(apiequality.Semantic.DeepEqual(generateArgs, expectArgs)).To(Equal(true))
		})
	}
}

func TestConstructBRGlobalOptionsForBackup(t *testing.T) {
	g := NewGomegaWithT(t)

	type testcase struct {
		name            string
		hasBackupFilter bool
		hasTable        bool
		hasDB           bool
	}

	tests := []*testcase{
		{
			name:            "empty filter, table and database",
			hasBackupFilter: false,
			hasTable:        false,
			hasDB:           false,
		},
		{
			name:            "customize filter, empty table and database",
			hasBackupFilter: true,
			hasTable:        false,
			hasDB:           false,
		},
		{
			name:            "empty filter, customize table and empty database",
			hasBackupFilter: false,
			hasTable:        true,
			hasDB:           false,
		},
		{
			name:            "empty filter, empty table and customize database",
			hasBackupFilter: false,
			hasTable:        false,
			hasDB:           true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			backup := newBackup()

			customBackupFilter := []string{"mysql.*"}
			customTable := []string{"table1"}
			customDb := []string{"mysql"}

			backup.Spec.BR = &v1alpha1.BRConfig{Cluster: "cluster-1", ClusterNamespace: "default"}
			var expectArgs []string
			expectArgs = append(expectArgs, "--storage=s3://test1-demo1")
			expectArgs = append(expectArgs, "--s3.provider=ceph")
			expectArgs = append(expectArgs, "--s3.endpoint=http://10.0.0.1")

			if tt.hasBackupFilter {
				backup.Spec.TableFilter = customBackupFilter
				expectArgs = append(expectArgs, "--filter", customBackupFilter[0])
			}

			if tt.hasTable {
				backup.Spec.Type = v1alpha1.BackupTypeTable
				backup.Spec.BR.Table = customTable[0]
				backup.Spec.BR.DB = customDb[0]
				expectArgs = append(expectArgs, fmt.Sprintf("--table=%s", customTable[0]))
				expectArgs = append(expectArgs, fmt.Sprintf("--db=%s", customDb[0]))
			}

			if tt.hasDB {
				backup.Spec.Type = v1alpha1.BackupTypeDB
				backup.Spec.BR.DB = customDb[0]
				expectArgs = append(expectArgs, fmt.Sprintf("--db=%s", customDb[0]))
			}

			generateArgs, err := ConstructBRGlobalOptionsForBackup(backup)
			g.Expect(err).To(Succeed())
			g.Expect(apiequality.Semantic.DeepEqual(generateArgs, expectArgs)).To(Equal(true))
		})
	}
}

func TestGetRemotePath(t *testing.T) {
	g := NewGomegaWithT(t)

	type testcase struct {
		name   string
		backup *v1alpha1.Backup
		expect string
		err    bool
	}

	tests := []*testcase{
		{
			name: "normal s3",
			backup: &v1alpha1.Backup{
				Spec: v1alpha1.BackupSpec{
					StorageProvider: v1alpha1.StorageProvider{
						S3: &v1alpha1.S3StorageProvider{
							Bucket:     "test1-demo1",
							SecretName: "demo",
						},
					},
				},
			},
			expect: "s3://test1-demo1",
		},
		{
			name: "normal gcs",
			backup: &v1alpha1.Backup{
				Spec: v1alpha1.BackupSpec{
					StorageProvider: v1alpha1.StorageProvider{
						Gcs: &v1alpha1.GcsStorageProvider{
							Bucket:     "test1-demo1",
							SecretName: "demo",
						},
					},
				},
			},
			expect: "gcs://test1-demo1/",
		},
		{
			name: "unknow storage type",
			backup: &v1alpha1.Backup{
				Spec: v1alpha1.BackupSpec{
					StorageProvider: v1alpha1.StorageProvider{},
				},
			},
			err: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p, err := GetStoragePath(tt.backup)
			if tt.err {
				g.Expect(err).To(HaveOccurred())
				return
			}
			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(p).To(Equal(tt.expect))
		})
	}
}

func TestSuffix(t *testing.T) {
	g := NewGomegaWithT(t)

	type testcase struct {
		name    string
		version string
		expect  string
	}

	tests := []*testcase{
		{
			name:    "normal",
			version: "v3.1.2",
			expect:  "31",
		},
		{
			name:    "unsupported version",
			version: "v1.2.3",
			expect:  "40",
		},
		{
			name:    "invalid version",
			version: "v4.0.x",
			expect:  "40",
		},
		{
			name:    "dirty version",
			version: "v4.0.0-20200909",
			expect:  "40",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			suffix := Suffix(tt.version)
			g.Expect(suffix).To(Equal(tt.expect))
		})
	}
}

func TestConstructBRGlobalOptionsForRestore(t *testing.T) {
	g := NewGomegaWithT(t)

	type testcase struct {
		name             string
		hasRestoreFilter bool
		hasTable         bool
		hasDB            bool
	}

	tests := []*testcase{
		{
			name:             "empty filter, table and database",
			hasRestoreFilter: false,
			hasTable:         false,
			hasDB:            false,
		},
		{
			name:             "customize filter, empty table and database",
			hasRestoreFilter: true,
			hasTable:         false,
			hasDB:            false,
		},
		{
			name:             "empty filter, customize table and empty database",
			hasRestoreFilter: false,
			hasTable:         true,
			hasDB:            false,
		},
		{
			name:             "empty filter, empty table and customize database",
			hasRestoreFilter: false,
			hasTable:         false,
			hasDB:            true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			restore := newRestore()

			customBackupFilter := []string{"mysql.*"}
			customTable := []string{"table1"}
			customDb := []string{"mysql"}

			restore.Spec.BR = &v1alpha1.BRConfig{Cluster: "cluster-1", ClusterNamespace: "default"}
			var expectArgs []string
			expectArgs = append(expectArgs, "--storage=s3://test1-demo1")
			expectArgs = append(expectArgs, "--s3.provider=ceph")
			expectArgs = append(expectArgs, "--s3.endpoint=http://10.0.0.1")

			if tt.hasRestoreFilter {
				restore.Spec.TableFilter = customBackupFilter
				expectArgs = append(expectArgs, "--filter", customBackupFilter[0])
			}

			if tt.hasTable {
				restore.Spec.Type = v1alpha1.BackupTypeTable
				restore.Spec.BR.Table = customTable[0]
				restore.Spec.BR.DB = customDb[0]
				expectArgs = append(expectArgs, fmt.Sprintf("--table=%s", customTable[0]))
				expectArgs = append(expectArgs, fmt.Sprintf("--db=%s", customDb[0]))
			}

			if tt.hasDB {
				restore.Spec.Type = v1alpha1.BackupTypeDB
				restore.Spec.BR.DB = customDb[0]
				expectArgs = append(expectArgs, fmt.Sprintf("--db=%s", customDb[0]))
			}

			generateArgs, err := ConstructBRGlobalOptionsForRestore(restore)
			g.Expect(err).To(Succeed())
			g.Expect(apiequality.Semantic.DeepEqual(generateArgs, expectArgs)).To(Equal(true))
		})
	}
}

func TestGetCommitTsFromMetadata(t *testing.T) {
	g := NewGomegaWithT(t)
	tmpdir, err := ioutil.TempDir("", "test-get-commitTs-metadata")
	g.Expect(err).To(Succeed())

	defer os.RemoveAll(tmpdir)
	metaDataFileName := filepath.Join(tmpdir, appconstant.MetaDataFile)

	err = ioutil.WriteFile(metaDataFileName, []byte(`Started dump at: 2019-06-13 10:00:04
		SHOW MASTER STATUS:
			Log: tidb-binlog
			Pos: 409054741514944513
			GTID:

		Finished dump at: 2019-06-13 10:00:04`), 0644)
	g.Expect(err).To(Succeed())

	commitTs, err := GetCommitTsFromMetadata(tmpdir)
	g.Expect(err).To(Succeed())
	g.Expect(commitTs).To(Equal("409054741514944513"))
}

func TestConstructRcloneArgs(t *testing.T) {
	g := NewGomegaWithT(t)

	type testcase struct {
		name       string
		config     string
		opts       []string
		command    string
		source     string
		dest       string
		verboseLog bool
		expect     []string
	}

	tests := []*testcase{
		{
			name:       "rclonels_wo_opts",
			config:     appconstant.RcloneConfigArg,
			opts:       nil,
			command:    "ls",
			source:     "src",
			dest:       "dst",
			verboseLog: false,
			expect:     []string{"--config=/tmp/rclone.conf", "ls", "src", "dst"},
		},
		{
			name:       "rclonels_w_opts",
			config:     appconstant.RcloneConfigArg,
			opts:       []string{"-v", "-vv", "--verbose=4", "-v=4", "--ignore-checksum"},
			command:    "ls",
			source:     "src",
			dest:       "dst",
			verboseLog: false,
			expect:     []string{"--config=/tmp/rclone.conf", "--ignore-checksum", "ls", "src", "dst"},
		},
		{
			name:       "rclonels_w_q",
			config:     appconstant.RcloneConfigArg,
			opts:       []string{"-q"},
			command:    "ls",
			source:     "src",
			dest:       "dst",
			verboseLog: false,
			expect:     []string{"--config=/tmp/rclone.conf", "-q", "ls", "src", "dst"},
		},
		{
			name:       "rclonecp_wo_opts",
			config:     appconstant.RcloneConfigArg,
			opts:       nil,
			command:    "copyto",
			source:     "src",
			dest:       "dst",
			verboseLog: true,
			expect:     []string{"--config=/tmp/rclone.conf", "-v", "copyto", "src", "dst"},
		},
		{
			name:       "rclonecp_w_opts",
			config:     appconstant.RcloneConfigArg,
			opts:       []string{"-v", "-vv", "--verbose=4", "-v=4"},
			command:    "copyto",
			source:     "src",
			dest:       "dst",
			verboseLog: true,
			expect:     []string{"--config=/tmp/rclone.conf", "-v", "-vv", "--verbose=4", "-v=4", "-v", "copyto", "src", "dst"},
		},
		{
			name:       "rclonecp_w_q",
			config:     appconstant.RcloneConfigArg,
			opts:       []string{"-q"},
			command:    "copyto",
			source:     "src",
			dest:       "dst",
			verboseLog: true,
			expect:     []string{"--config=/tmp/rclone.conf", "-q", "copyto", "src", "dst"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			opts := ConstructRcloneArgs(tt.config, tt.opts, tt.command, tt.source, tt.dest, tt.verboseLog)
			g.Expect(opts).To(Equal(tt.expect))
		})
	}

}

func newBackup() *v1alpha1.Backup {
	return &v1alpha1.Backup{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Backup",
			APIVersion: "pingcap.com/v1alpha1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-backup",
			Namespace: corev1.NamespaceDefault,
			UID:       types.UID("test-bk"),
		},
		Spec: v1alpha1.BackupSpec{
			From: &v1alpha1.TiDBAccessConfig{
				Host:       "10.1.1.2",
				Port:       v1alpha1.DefaultTiDBServicePort,
				User:       v1alpha1.DefaultTidbUser,
				SecretName: "demo1-tidb-secret",
			},
			StorageProvider: v1alpha1.StorageProvider{
				S3: &v1alpha1.S3StorageProvider{
					Provider:   v1alpha1.S3StorageProviderTypeCeph,
					Endpoint:   "http://10.0.0.1",
					Bucket:     "test1-demo1",
					SecretName: "demo",
				},
			},
			StorageClassName: pointer.StringPtr("local-storage"),
			StorageSize:      "1Gi",
		},
	}
}

func newRestore() *v1alpha1.Restore {
	return &v1alpha1.Restore{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Restore",
			APIVersion: "pingcap.com/v1alpha1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-restore",
			Namespace: corev1.NamespaceDefault,
			UID:       types.UID("test-re"),
		},
		Spec: v1alpha1.RestoreSpec{
			To: &v1alpha1.TiDBAccessConfig{
				Host:       "10.1.1.2",
				Port:       v1alpha1.DefaultTiDBServicePort,
				User:       v1alpha1.DefaultTidbUser,
				SecretName: "demo1-tidb-secret",
			},
			StorageProvider: v1alpha1.StorageProvider{
				S3: &v1alpha1.S3StorageProvider{
					Provider:   v1alpha1.S3StorageProviderTypeCeph,
					Endpoint:   "http://10.0.0.1",
					Bucket:     "test1-demo1",
					SecretName: "demo",
				},
			},
			StorageClassName: pointer.StringPtr("local-storage"),
			StorageSize:      "1Gi",
		},
	}
}
