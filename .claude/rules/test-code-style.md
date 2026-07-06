---
paths:
  - "tests/**/*.go"
---

# SSP Operator Functional Test Style Guide

This rule applies when writing or modifying tests under the `tests/` directory.

## Test Framework

Tests use **Ginkgo v2** (BDD) with **Gomega** matchers, both dot-imported. All tests live in the `tests` package (not `tests_test`). The single entry point is `TestFunctional` in `tests_suite_test.go`.

## File Organization

- One `_test.go` file per operand or feature area (e.g., `commonTemplates_test.go`, `metrics_test.go`).
- Each file has one top-level `var _ = Describe("Feature area", func() { ... })`.
- Non-test helpers go in files without the `_test.go` suffix (e.g., `tests_common_test.go`, `watcher.go`, `port-forwarding.go`).

## Describe/Context/It Structure

```
Describe("Feature area")
  BeforeEach  ← initialize testResource vars + waitUntilDeployed()
  Context("resource creation")
    DescribeTable / It
  Context("resource change")
    DescribeTable / It
    Context("with pause")
      BeforeEach ← strategy.SkipSspUpdateTestsIfNeeded()
      JustAfterEach ← unpauseSsp()
      DescribeTable / It
  Context("resource deletion")
    DescribeTable / It
```

Use **`DescribeTable` + `Entry`** when the same test logic runs against multiple resource types. Place the `decorators.Conformance` decorator on `DescribeTable` when it applies to all entries, or on individual `Entry` calls otherwise.

Use **`Ordered`** containers only when setup is expensive and must run once (e.g., multi-arch tests). Replace `BeforeEach`/`AfterEach` with `BeforeAll`/`AfterAll` inside `Ordered` containers.

## Test Naming

- `Describe`: feature area noun — `"Common templates"`, `"Metrics"`, `"Validation webhook"`
- `Context`: narrowing condition — `"resource creation"`, `"with pause"`, `"with DataImportCron template"`
- `It` / `Entry`: imperative sentence — `"should restore modified resource"`, `"should set Deployed phase"`
- API version in description when covering multiple versions: `"[v1beta2] should fail to create..."`, `"[v1beta3] should fail to create..."`

## The `testResource` Struct

The central abstraction for any k8s object the operator manages:

```go
type testResource struct {
    Name           string
    Namespace      string
    Resource       client.Object        // zero-value instance; used for DeepCopy
    ExpectedLabels map[string]string
    UpdateFunc     interface{}          // func(*ConcreteType) — applied via reflection
    EqualsFunc     interface{}          // func(*ConcreteType, *ConcreteType) bool — applied via reflection
}
```

Always initialize `testResource` variables **inside `BeforeEach`**, never at package level, because `strategy.GetNamespace()` is only available at runtime.

## BeforeEach / AfterEach Patterns

**Top-level `BeforeEach`** always does two things:
1. Initialize all `testResource` structs for the describe block.
2. Call `waitUntilDeployed()` to ensure a clean operator state.

**SSP-modification contexts** always follow this pattern:

```go
// Inside Context("with SSP change"):
BeforeEach(func() {
    strategy.SkipSspUpdateTestsIfNeeded()
    // or: strategy.SkipIfUpgradeLane() / skipIfUpgradeLane()
})
AfterEach(func() {
    strategy.RevertToOriginalSspCr()
    waitUntilDeployed()
})
```

**Use `AfterEach`** for cleanup that must always run after a test, such as unpausing SSP:

```go
AfterEach(func() {
    unpauseSsp()
})
```

**Use `AfterEach`** for failure-time debug logging:

```go
AfterEach(func() {
    if CurrentSpecReport().Failed() {
        logObject(client.ObjectKeyFromObject(vm), &kubevirtv1.VirtualMachine{})
    }
})
```

**Cleaning up created resources** — wrap deletion in `Eventually` to tolerate NotFound races:

```go
AfterEach(func() {
    Expect(apiClient.Delete(ctx, obj)).To(
        Or(Succeed(), MatchError(k8serrors.IsNotFound, "errors.IsNotFound")))
    waitForDeletion(client.ObjectKeyFromObject(obj), obj)
})
```

## Modifying the SSP CR

Always use `updateSsp(func(foundSsp *ssp.SSP))`. It retries automatically on conflicts:

```go
updateSsp(func(foundSsp *ssp.SSP) {
    foundSsp.Spec.TemplateValidator = &ssp.TemplateValidator{
        Replicas: &newCount,
    }
})
waitUntilDeployed()
```

Never call `apiClient.Update` on the SSP CR directly in tests — always go through `updateSsp`.

For webhook validation tests, use `client.DryRunAll` to test admission without persisting:

```go
err := apiClient.Create(ctx, ssp2, client.DryRunAll)
Expect(err).To(MatchError(ContainSubstring("expected error text")))
```

## The Three Fundamental Reconciliation Tests

Every managed resource type should have these three tests, using the provided helpers:

1. **Recreate after delete** — `expectRecreateAfterDelete(&res)`
2. **Restore after update** — `expectRestoreAfterUpdate(&res)`
3. **Restore after update with pause** — `expectRestoreAfterUpdateWithPause(&res)` (requires SSP-modification guard)

And when resource carries app labels:

4. **App labels restored** — `expectAppLabelsRestoreAfterUpdate(&res)` (from `labels_test.go`)

## Assertion Patterns

### Timeouts

Always use `env.Timeout()` or `env.ShortTimeout()` — never hard-code durations except for short, specific sleeps or `Consistently` stability windows:

| Situation | Timeout |
|---|---|
| Waiting for SSP to deploy or reconcile fully | `env.Timeout()` (10 min) |
| Most `Eventually` assertions | `env.ShortTimeout()` (1 min) |
| `Consistently` stability checks | `20*time.Second` or `pauseDuration` |
| Prometheus metric stability | `2*time.Minute` with `MustPassRepeatedly(10)` |

Polling interval is always `time.Second` unless there is a specific reason otherwise.

### `Eventually` — preferred form

Use the `g Gomega` parameter form when making multiple assertions inside `Eventually`, to collect all failures rather than short-circuiting on the first:

```go
// Preferred
Eventually(func(g Gomega) {
    g.Expect(apiClient.Get(ctx, key, obj)).To(Succeed())
    g.Expect(obj.Status.Phase).To(Equal(expected))
}, env.ShortTimeout(), time.Second).Should(Succeed())

// Acceptable for single assertions
Eventually(func() error {
    return apiClient.Get(ctx, key, obj)
}, env.ShortTimeout(), time.Second).Should(Succeed())
```

### `Consistently`

Use to verify that the operator does NOT act during a pause or after a non-spec change:

```go
Consistently(func(g Gomega) {
    found := res.NewResource()
    g.Expect(apiClient.Get(ctx, res.GetKey(), found)).To(Succeed())
    g.Expect(found).To(EqualResource(&res, changed))
}, 20*time.Second, time.Second).Should(Succeed())
```

### `MustPassRepeatedly`

Use for anti-flake in monitoring/alert tests to ensure a condition is stable, not transient:

```go
Eventually(..., env.Timeout(), 10*time.Second).MustPassRepeatedly(10).Should(...)
```

### Error matching

```go
// Preferred: predicate form
Expect(err).To(MatchError(k8serrors.IsNotFound, "k8serrors.IsNotFound"))

// For API errors by reason
Expect(k8serrors.ReasonForError(err)).To(Equal(metav1.StatusReasonNotFound))

// For message content
Expect(err).To(MatchError(ContainSubstring("expected text")))

// For either success or not-found (in cleanup)
Expect(err).To(Or(Succeed(), MatchError(k8serrors.IsNotFound, "k8serrors.IsNotFound")))
```

## Decorators and Skips

Use `decorators.Conformance` (which is `Label("conformance")`) for tests that must pass in all deployment configurations:

```go
DescribeTable("created cluster resource", decorators.Conformance, func(res *testResource) { ... },
    Entry("[test_id:4584]cluster role", &clusterRoleRes),
)
```

For conditional skips, call strategy methods inside `BeforeEach` — do not skip via decorators:

```go
BeforeEach(func() {
    strategy.SkipSspUpdateTestsIfNeeded()             // when SKIP_UPDATE_SSP_TESTS=true
    strategy.SkipUnlessHighlyAvailableTopologyMode()  // HA-only tests
    strategy.SkipUnlessSingleReplicaTopologyMode()    // single-node tests
    strategy.SkipIfUpgradeLane()                      // breaks upgrades
    skipIfUpgradeLane()                               // bare function variant
})
```

## Environment Variables

Never access env vars directly — always use the `env` package:

```go
env.Timeout()            // TIMEOUT_MINUTES, default 10m
env.ShortTimeout()       // SHORT_TIMEOUT_MINUTES, default 1m
env.TopologyMode()       // TOPOLOGY_MODE
env.IsUpgradeLane()      // IS_UPGRADE_LANE
env.SkipUpdateSspTests() // SKIP_UPDATE_SSP_TESTS
```

## Triggering Reconciliation

To force the operator to reconcile without changing spec, use `triggerReconciliation()`. It adds and removes a dummy annotation, then calls `waitUntilDeployed()`. Use it when a test needs to verify behavior after a non-spec event.

To observe the precise sequence of status transitions (not just the final state), use the `SspWatch`:

```go
watch, err := StartWatch(sspListerWatcher)
Expect(err).ToNot(HaveOccurred())
defer watch.Stop()

// trigger change ...

err = WatchChangesUntil(watch, func(ssp *sspv1beta3.SSP) bool {
    return ssp.Generation > ssp.Status.ObservedGeneration
}, env.ShortTimeout())
Expect(err).ToNot(HaveOccurred())
```

## Metrics and Monitoring Tests

- Read metrics from operator pods via port-forward using `collectMetricFromOperator(metricName)`.
- Always capture a baseline count before the action: `countBefore, err := sspOperatorReconcileSucceededCount()`.
- Verify exact delta, not absolute value: `Expect(countAfter - countBefore).To(Equal(1))`.
- For alert tests: wait for series (`waitForSeriesToBeDetected`), trigger condition, wait for alert (`waitForAlertToActivate`), then verify absence with `MustPassRepeatedly(10)` (`alertShouldNotBeActive`).

## Global Variables

These are set in `BeforeSuite` and available in all test files:

```go
var (
    apiClient        client.Client          // controller-runtime client for all k8s operations
    coreClient       *kubernetes.Clientset  // go-client (for pods, logs, etc.)
    ctx              context.Context
    strategy         TestSuiteStrategy      // newSsp or existingSsp
    sspListerWatcher cache.ListerWatcherWithContext
    portForwarder    PortForwarder
    nodeArchitecture string                 // "amd64", "arm64", etc.
    templatesSuffix  string                 // derived from nodeArchitecture
)
```

Do not declare new package-level mutable variables. All test-local state goes in `BeforeEach`-scoped local variables.
