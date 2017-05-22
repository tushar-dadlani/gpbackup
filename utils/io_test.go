package utils_test

import (
	"database/sql"
	"database/sql/driver"
	"errors"
	"gpbackup/backup"
	"gpbackup/testutils"
	"gpbackup/utils"
	"io/ioutil"
	"os"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"gopkg.in/DATA-DOG/go-sqlmock.v1"
)

var _ = Describe("utils/io tests", func() {
	var connection *utils.DBConn
	var mock sqlmock.Sqlmock

	BeforeEach(func() {
		connection, mock = testutils.CreateAndConnectMockDB()
		testutils.SetupTestLogger()
		utils.DumpTimestamp = "20170101010101"
	})

	Describe("DirectoryMustExist", func() {
		It("does nothing if the directory exists", func() {
			utils.System.Stat = func(name string) (os.FileInfo, error) { return nil, nil }
			defer func() { utils.System.Stat = os.Stat }()
			utils.DirectoryMustExist("dirname")
		})
		It("panics if the directory does not exist", func() {
			utils.System.Stat = func(name string) (os.FileInfo, error) { return nil, errors.New("No such file or directory") }
			defer func() { utils.System.Stat = os.Stat }()
			defer testutils.ShouldPanicWithMessage("Cannot stat directory dirname: No such file or directory")
			utils.DirectoryMustExist("dirname")
		})
	})
	Describe("MustOpenFile", func() {
		It("creates or opens the file for writing", func() {
			utils.System.Create = func(name string) (*os.File, error) { return os.Stdout, nil }
			defer func() { utils.System.Create = os.Create }()
			fileHandle := utils.MustOpenFile("filename")
			Expect(fileHandle).To(Equal(os.Stdout))
		})
		It("panics on error", func() {
			utils.System.Create = func(name string) (*os.File, error) { return nil, errors.New("Permission denied") }
			defer func() { utils.System.Create = os.Create }()
			defer testutils.ShouldPanicWithMessage("Unable to create or open file filename: Permission denied")
			utils.MustOpenFile("filename")
		})
	})
	Describe("GetSegmentConfiguration", func() {
		header := []string{"content", "hostname", "datadir"}
		localSegOne := []driver.Value{"0", "localhost", "/data/gpseg0"}
		localSegTwo := []driver.Value{"1", "localhost", "/data/gpseg1"}
		remoteSegOne := []driver.Value{"2", "remotehost", "/data/gpseg2"}
		BeforeEach(func() {
			utils.BaseDumpDir = "<SEG_DATA_DIR>"
		})

		It("returns a configuration for a single-host, single-segment cluster", func() {
			fakeResult := sqlmock.NewRows(header).AddRow(localSegOne...)
			mock.ExpectQuery("SELECT (.*)").WillReturnRows(fakeResult)
			results := utils.GetSegmentConfiguration(connection)
			Expect(len(results)).To(Equal(1))
			Expect(results[0].DataDir).To(Equal("/data/gpseg0"))
			Expect(results[0].Hostname).To(Equal("localhost"))
		})
		It("returns a configuration for a single-host, multi-segment cluster", func() {
			fakeResult := sqlmock.NewRows(header).AddRow(localSegOne...).AddRow(localSegTwo...)
			mock.ExpectQuery("SELECT (.*)").WillReturnRows(fakeResult)
			results := utils.GetSegmentConfiguration(connection)
			Expect(len(results)).To(Equal(2))
			Expect(results[0].DataDir).To(Equal("/data/gpseg0"))
			Expect(results[0].Hostname).To(Equal("localhost"))
			Expect(results[1].DataDir).To(Equal("/data/gpseg1"))
			Expect(results[1].Hostname).To(Equal("localhost"))
		})
		It("returns a configuration for a multi-host, multi-segment cluster", func() {
			fakeResult := sqlmock.NewRows(header).AddRow(localSegOne...).AddRow(localSegTwo...).AddRow(remoteSegOne...)
			mock.ExpectQuery("SELECT (.*)").WillReturnRows(fakeResult)
			results := utils.GetSegmentConfiguration(connection)
			Expect(len(results)).To(Equal(3))
			Expect(results[0].DataDir).To(Equal("/data/gpseg0"))
			Expect(results[0].Hostname).To(Equal("localhost"))
			Expect(results[1].DataDir).To(Equal("/data/gpseg1"))
			Expect(results[1].Hostname).To(Equal("localhost"))
			Expect(results[2].DataDir).To(Equal("/data/gpseg2"))
			Expect(results[2].Hostname).To(Equal("remotehost"))
		})
	})
	Describe("SetupSegmentConfiguration", func() {
		masterSeg := utils.QuerySegConfig{-1, "localhost", "/data/gpseg-1"}
		localSegOne := utils.QuerySegConfig{0, "localhost", "/data/gpseg0"}
		localSegTwo := utils.QuerySegConfig{1, "localhost", "/data/gpseg1"}
		remoteSegTwo := utils.QuerySegConfig{1, "remotehost", "/data/gpseg1"}

		It("sets up the configuration for a single-host, single-segment cluster", func() {
			utils.SetupSegmentConfiguration([]utils.QuerySegConfig{masterSeg, localSegOne})
			Expect(len(utils.GetContentList())).To(Equal(2))
			Expect(utils.GetDirForContent(-1)).To(Equal("/data/gpseg-1/backups/20170101/20170101010101"))
			Expect(utils.GetHostForContent(-1)).To(Equal("localhost"))
			Expect(utils.GetDirForContent(0)).To(Equal("/data/gpseg0/backups/20170101/20170101010101"))
			Expect(utils.GetHostForContent(0)).To(Equal("localhost"))
		})
		It("sets up the configuration for a single-host, multi-segment cluster", func() {
			utils.SetupSegmentConfiguration([]utils.QuerySegConfig{masterSeg, localSegOne, localSegTwo})
			Expect(len(utils.GetContentList())).To(Equal(3))
			Expect(utils.GetDirForContent(-1)).To(Equal("/data/gpseg-1/backups/20170101/20170101010101"))
			Expect(utils.GetHostForContent(-1)).To(Equal("localhost"))
			Expect(utils.GetDirForContent(0)).To(Equal("/data/gpseg0/backups/20170101/20170101010101"))
			Expect(utils.GetHostForContent(0)).To(Equal("localhost"))
			Expect(utils.GetDirForContent(1)).To(Equal("/data/gpseg1/backups/20170101/20170101010101"))
			Expect(utils.GetHostForContent(1)).To(Equal("localhost"))
		})
		It("sets up the configuration for a multi-host, multi-segment cluster", func() {
			utils.SetupSegmentConfiguration([]utils.QuerySegConfig{masterSeg, localSegOne, remoteSegTwo})
			Expect(len(utils.GetContentList())).To(Equal(3))
			Expect(utils.GetDirForContent(-1)).To(Equal("/data/gpseg-1/backups/20170101/20170101010101"))
			Expect(utils.GetHostForContent(-1)).To(Equal("localhost"))
			Expect(utils.GetDirForContent(0)).To(Equal("/data/gpseg0/backups/20170101/20170101010101"))
			Expect(utils.GetHostForContent(0)).To(Equal("localhost"))
			Expect(utils.GetDirForContent(1)).To(Equal("/data/gpseg1/backups/20170101/20170101010101"))
			Expect(utils.GetHostForContent(1)).To(Equal("remotehost"))
		})
	})
	Describe("CreateDumpDirs", func() {
		It("creates directories relative to the segment data directory", func() {
			checkMap := make(map[string]bool, 0)
			utils.System.MkdirAll = func(path string, perm os.FileMode) error {
				checkMap[path] = true
				Expect(perm).To(Equal(os.FileMode(0700)))
				return nil
			}
			defer func() { utils.System.MkdirAll = os.MkdirAll }()
			testutils.SetDefaultSegmentConfiguration()
			utils.CreateDumpDirs()
			Expect(len(checkMap)).To(Equal(3))
			Expect(checkMap["/data/gpseg-1/backups/20170101/20170101010101"]).To(BeTrue())
			Expect(checkMap["/data/gpseg0/backups/20170101/20170101010101"]).To(BeTrue())
			Expect(checkMap["/data/gpseg1/backups/20170101/20170101010101"]).To(BeTrue())
		})
		It("creates directories relative to a user-specified directory", func() {
			checkMap := make(map[string]bool, 0)
			utils.System.MkdirAll = func(path string, perm os.FileMode) error {
				checkMap[path] = true
				Expect(perm).To(Equal(os.FileMode(0700)))
				return nil
			}
			defer func() { utils.System.MkdirAll = os.MkdirAll; utils.BaseDumpDir = utils.DefaultSegmentDir }()
			utils.BaseDumpDir = "/tmp/foo"
			configMaster := utils.QuerySegConfig{-1, "/data/gpseg-1", "localhost"}
			configSegOne := utils.QuerySegConfig{0, "/data/gpseg0", "localhost"}
			configSegTwo := utils.QuerySegConfig{1, "/data/gpseg1", "localhost"}
			utils.SetupSegmentConfiguration([]utils.QuerySegConfig{configMaster, configSegOne, configSegTwo})
			utils.CreateDumpDirs()
			Expect(len(checkMap)).To(Equal(1))
			Expect(checkMap["/tmp/foo/backups/20170101/20170101010101"]).To(BeTrue())
		})
	})
	Describe("WriteTableMapFile", func() {
		testutils.SetDefaultSegmentConfiguration()
		tableOne := utils.Relation{0, 1234, "public", "foo", sql.NullString{"", false}}
		tableTwo := utils.Relation{0, 2345, "public", "foo|bar", sql.NullString{"", false}}

		It("writes a map file containing one table", func() {
			filePath := ""
			r, w, _ := os.Pipe()
			utils.System.Create = func(name string) (*os.File, error) { filePath = name; return w, nil }
			defer func() { utils.System.Create = os.Create }()
			tables := []utils.Relation{tableOne}
			backup.WriteTableMapFile(tables)
			w.Close()
			output, _ := ioutil.ReadAll(r)
			testutils.ExpectRegex(string(output), `public.foo: 1234
`)
		})
		It("writes a map file containing multiple tables", func() {
			filePath := ""
			r, w, _ := os.Pipe()
			utils.System.Create = func(name string) (*os.File, error) { filePath = name; return w, nil }
			defer func() { utils.System.Create = os.Create }()
			tables := []utils.Relation{tableOne, tableTwo}
			backup.WriteTableMapFile(tables)
			w.Close()
			output, _ := ioutil.ReadAll(r)
			testutils.ExpectRegex(string(output), `public.foo: 1234
public."foo|bar": 2345`)
		})
	})
})
