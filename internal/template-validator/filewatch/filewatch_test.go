package filewatch

import (
	"os"
	"path/filepath"
	"runtime"
	"sync/atomic"
	"testing"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Filewatch", func() {
	It("should call callback immediately when watch starts", func() {
		tempDirName := GinkgoT().TempDir()

		called := atomic.Bool{}
		startWatch(tempDirName, func() { called.Store(true) })

		Eventually(called.Load, time.Second, 50*time.Millisecond).Should(BeTrue())
	})

	Context("watching file", func() {
		var (
			callback     func()
			tempFileName string
		)

		BeforeEach(func() {
			tmpDir := GinkgoT().TempDir()
			tempFileName = filepath.Join(tmpDir, "test-file")
			Expect(os.WriteFile(tempFileName, []byte("test content"), 0777)).To(Succeed())

			callback = func() {}

			startWatch(tempFileName, func() { callback() })
		})

		It("should call callback on file change", func() {
			called := atomic.Bool{}
			callback = func() { called.Store(true) }

			Expect(os.WriteFile(tempFileName, []byte("different content"), 0777)).To(Succeed())

			Eventually(called.Load, time.Second, 50*time.Millisecond).Should(BeTrue())
		})

		It("should call callback on file deletion", func() {
			called := atomic.Bool{}
			callback = func() { called.Store(true) }

			Expect(os.Remove(tempFileName)).ToNot(HaveOccurred())

			Eventually(called.Load, time.Second, 50*time.Millisecond).Should(BeTrue())
		})
	})

	Context("watching directory", func() {
		var (
			callback    func()
			tempDirName string
		)

		BeforeEach(func() {
			tempDirName = GinkgoT().TempDir()

			callback = func() {}

			startWatch(tempDirName, func() { callback() })
		})

		It("should call callback on file creation", func() {
			called := atomic.Bool{}
			callback = func() { called.Store(true) }

			Expect(os.WriteFile(filepath.Join(tempDirName, "created-file"), []byte("content"), 0777)).To(Succeed())

			Eventually(called.Load, time.Second, 50*time.Millisecond).Should(BeTrue())
		})

		It("should call callback on file change", func() {
			called := atomic.Bool{}
			callback = func() { called.Store(true) }

			const filename = "test-file"
			Expect(os.WriteFile(filepath.Join(tempDirName, filename), []byte("content"), 0777)).To(Succeed())

			Eventually(called.Load, time.Second, 50*time.Millisecond).Should(BeTrue())

			called.Store(false)

			Expect(os.WriteFile(filepath.Join(tempDirName, filename), []byte("updated content"), 0777)).To(Succeed())

			Eventually(called.Load, time.Second, 50*time.Millisecond).Should(BeTrue())
		})

		It("should call callback on file deletion", func() {
			called := atomic.Bool{}
			callback = func() { called.Store(true) }

			const filename = "test-file"
			Expect(os.WriteFile(filepath.Join(tempDirName, filename), []byte("content"), 0777)).To(Succeed())

			Eventually(called.Load, time.Second, 50*time.Millisecond).Should(BeTrue())

			called.Store(false)

			Expect(os.Remove(filepath.Join(tempDirName, filename))).ToNot(HaveOccurred())

			Eventually(called.Load, time.Second, 50*time.Millisecond).Should(BeTrue())
		})
	})
})

func startWatch(path string, callback func()) {
	testWatch := New()
	err := testWatch.Add(path, callback)
	Expect(err).ToNot(HaveOccurred())

	done := make(chan struct{})
	DeferCleanup(func() {
		close(done)
	})

	go func() {
		defer GinkgoRecover()
		Expect(testWatch.Run(done)).To(Succeed())
	}()

	// Wait for a short time to let the watch goroutine run
	runtime.Gosched()
	Eventually(testWatch.IsRunning, time.Second, 50*time.Millisecond).Should(BeTrue())
}

func TestFilewatch(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Filewatch Suite")
}
