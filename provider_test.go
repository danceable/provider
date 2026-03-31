package provider_test

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/danceable/container/bind"
	"github.com/danceable/container/resolve"
	"github.com/danceable/provider"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

type containerMock struct {
	mock.Mock
}

func (m *containerMock) Reset() { m.Called() }

func (m *containerMock) Bind(r any, opts ...bind.BindOption) error { return m.Called(r, opts).Error(0) }

func (m *containerMock) Call(r any, opts ...resolve.ResolveOption) error {
	return m.Called(r, opts).Error(0)
}

func (m *containerMock) Resolve(a any, opts ...resolve.ResolveOption) error {
	return m.Called(a, opts).Error(0)
}

func (m *containerMock) Fill(r any, opts ...resolve.ResolveOption) error {
	return m.Called(r, opts).Error(0)
}

type providerMock struct {
	mock.Mock
}

func (p *providerMock) Register(ctx context.Context, c provider.Container) error {
	return p.Called(ctx, c).Error(0)
}

func (p *providerMock) Boot(ctx context.Context, c provider.Container) error {
	return p.Called(ctx, c).Error(0)
}

func (p *providerMock) Terminate(ctx context.Context) error {
	return p.Called(ctx).Error(0)
}

type orderedProviderMock struct {
	providerMock
	order int
}

func (p *orderedProviderMock) Order() int {
	return p.order
}

func setupProviderMock(p *providerMock, name string, calls *[]string, regErr, bootErr, termErr error) {
	regCall := p.On("Register", mock.Anything, mock.Anything).Return(regErr)
	if calls != nil {
		regCall.Run(func(_ mock.Arguments) { *calls = append(*calls, name+".Register") })
	}

	bootCall := p.On("Boot", mock.Anything, mock.Anything).Return(bootErr)
	if calls != nil {
		bootCall.Run(func(_ mock.Arguments) { *calls = append(*calls, name+".Boot") })
	}

	termCall := p.On("Terminate", mock.Anything).Return(termErr)
	if calls != nil {
		termCall.Run(func(_ mock.Arguments) { *calls = append(*calls, name+".Terminate") })
	}
}

func TestNew(t *testing.T) {
	t.Parallel()

	m := provider.New(new(containerMock))
	require.NotNil(t, m)
}

func TestRegister_WithoutOrder(t *testing.T) {
	t.Parallel()

	m := provider.New(new(containerMock))

	var mocks [3]*providerMock
	for i := range mocks {
		p := new(providerMock)
		setupProviderMock(p, "", nil, nil, nil, nil)
		mocks[i] = p
		m.Register(p)
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	require.NoError(t, m.Run(ctx))

	for _, p := range mocks {
		p.AssertCalled(t, "Register", mock.Anything, mock.Anything)
		p.AssertCalled(t, "Boot", mock.Anything, mock.Anything)
		p.AssertCalled(t, "Terminate", mock.Anything)
	}
}

func TestRegister_WithOrder(t *testing.T) {
	t.Parallel()

	var calls []string
	m := provider.New(new(containerMock))

	p1 := &orderedProviderMock{order: 10}
	setupProviderMock(&p1.providerMock, "p1", &calls, nil, nil, nil)
	p2 := &orderedProviderMock{order: 5}
	setupProviderMock(&p2.providerMock, "p2", &calls, nil, nil, nil)
	p3 := &orderedProviderMock{order: 10}
	setupProviderMock(&p3.providerMock, "p3", &calls, nil, nil, nil)

	m.Register(p1)
	m.Register(p2)
	m.Register(p3)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	require.NoError(t, m.Run(ctx))

	// Order 5 (p2) should execute before order 10 (p1, p3)
	assert.Equal(t, []string{
		"p2.Register",
		"p1.Register",
		"p3.Register",
		"p2.Boot",
		"p1.Boot",
		"p3.Boot",
		"p3.Terminate",
		"p1.Terminate",
		"p2.Terminate",
	}, calls)
}

func TestRegister_MixedOrderAndUnordered(t *testing.T) {
	t.Parallel()

	m := provider.New(new(containerMock))

	ordered := &orderedProviderMock{order: 5}
	setupProviderMock(&ordered.providerMock, "", nil, nil, nil, nil)

	unordered := new(providerMock)
	setupProviderMock(unordered, "", nil, nil, nil, nil)

	m.Register(ordered)
	m.Register(unordered)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	require.NoError(t, m.Run(ctx))

	ordered.AssertExpectations(t)
	unordered.AssertExpectations(t)
}

func TestRun_LifecycleOrder(t *testing.T) {
	t.Parallel()

	var calls []string
	m := provider.New(new(containerMock))

	p1 := &orderedProviderMock{order: 1}
	setupProviderMock(&p1.providerMock, "p1", &calls, nil, nil, nil)

	p2 := &orderedProviderMock{order: 2}
	setupProviderMock(&p2.providerMock, "p2", &calls, nil, nil, nil)

	m.Register(p1)
	m.Register(p2)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := m.Run(ctx)
	require.NoError(t, err)

	assert.Equal(t, []string{
		"p1.Register",
		"p2.Register",
		"p1.Boot",
		"p2.Boot",
		"p2.Terminate",
		"p1.Terminate",
	}, calls)
}

func TestRun_RegisterError(t *testing.T) {
	t.Parallel()

	m := provider.New(new(containerMock))
	regErr := errors.New("register failed")

	p := new(providerMock)
	setupProviderMock(p, "", nil, regErr, nil, nil)
	m.Register(p)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	err := m.Run(ctx)
	assert.ErrorIs(t, err, regErr)
}

func TestRun_BootError(t *testing.T) {
	t.Parallel()

	m := provider.New(new(containerMock))
	bootErr := errors.New("boot failed")

	p := new(providerMock)
	setupProviderMock(p, "", nil, nil, bootErr, nil)
	m.Register(p)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	err := m.Run(ctx)
	assert.ErrorIs(t, err, bootErr)
}

func TestRun_TerminateError(t *testing.T) {
	t.Parallel()

	m := provider.New(new(containerMock))
	termErr := errors.New("terminate failed")

	p := new(providerMock)
	setupProviderMock(p, "", nil, nil, nil, termErr)
	m.Register(p)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := m.Run(ctx)
	assert.ErrorIs(t, err, termErr)
}

func TestRun_TerminateReverseOrder(t *testing.T) {
	t.Parallel()

	var calls []string
	m := provider.New(new(containerMock))

	p1 := &orderedProviderMock{order: 1}
	setupProviderMock(&p1.providerMock, "first", &calls, nil, nil, nil)

	p2 := &orderedProviderMock{order: 2}
	setupProviderMock(&p2.providerMock, "second", &calls, nil, nil, nil)

	p3 := &orderedProviderMock{order: 3}
	setupProviderMock(&p3.providerMock, "third", &calls, nil, nil, nil)

	m.Register(p1)
	m.Register(p2)
	m.Register(p3)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := m.Run(ctx)
	require.NoError(t, err)

	var terminateCalls []string
	for _, c := range calls {
		if strings.HasSuffix(c, ".Terminate") {
			terminateCalls = append(terminateCalls, c)
		}
	}

	assert.Equal(t, []string{"third.Terminate", "second.Terminate", "first.Terminate"}, terminateCalls)
}

func TestRun_MultipleProvidersAtSameOrder(t *testing.T) {
	t.Parallel()

	var calls []string
	m := provider.New(new(containerMock))

	pA := &orderedProviderMock{order: 1}
	setupProviderMock(&pA.providerMock, "A", &calls, nil, nil, nil)

	pB := &orderedProviderMock{order: 1}
	setupProviderMock(&pB.providerMock, "B", &calls, nil, nil, nil)

	m.Register(pA)
	m.Register(pB)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := m.Run(ctx)
	require.NoError(t, err)

	assert.Equal(t, []string{
		"A.Register",
		"B.Register",
		"A.Boot",
		"B.Boot",
		"B.Terminate",
		"A.Terminate",
	}, calls)
}

func TestRun_NoProviders(t *testing.T) {
	t.Parallel()

	m := provider.New(new(containerMock))

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := m.Run(ctx)
	assert.NoError(t, err)
}

func TestRun_BootErrorStopsEarly(t *testing.T) {
	t.Parallel()

	var calls []string
	m := provider.New(new(containerMock))

	p1 := &orderedProviderMock{order: 1}
	setupProviderMock(&p1.providerMock, "p1", &calls, nil, nil, nil)

	p2 := &orderedProviderMock{order: 2}
	setupProviderMock(&p2.providerMock, "p2", &calls, nil, errors.New("boot fail"), nil)

	p3 := &orderedProviderMock{order: 3}
	setupProviderMock(&p3.providerMock, "p3", &calls, nil, nil, nil)

	m.Register(p1)
	m.Register(p2)
	m.Register(p3)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	err := m.Run(ctx)
	require.Error(t, err)

	assert.NotContains(t, calls, "p3.Boot")
	assert.Contains(t, calls, "p1.Boot")
}

func TestRun_RegisterErrorStopsEarly(t *testing.T) {
	t.Parallel()

	var calls []string
	m := provider.New(new(containerMock))

	p1 := &orderedProviderMock{order: 1}
	setupProviderMock(&p1.providerMock, "p1", &calls, errors.New("reg fail"), nil, nil)

	p2 := &orderedProviderMock{order: 2}
	setupProviderMock(&p2.providerMock, "p2", &calls, nil, nil, nil)

	m.Register(p1)
	m.Register(p2)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	err := m.Run(ctx)
	require.Error(t, err)

	assert.NotContains(t, calls, "p1.Boot")
	assert.NotContains(t, calls, "p2.Boot")
	assert.NotContains(t, calls, "p2.Register")
}

func TestRun_CallbackInvoked(t *testing.T) {
	t.Parallel()

	m := provider.New(new(containerMock))

	p := new(providerMock)
	setupProviderMock(p, "", nil, nil, nil, nil)
	m.Register(p)

	ctx, cancel := context.WithCancel(context.Background())
	callbackCalled := make(chan struct{})

	err := m.Run(ctx, provider.WithCallback(func(ctx context.Context, c provider.Container) {
		close(callbackCalled)
		cancel()
	}))

	require.NoError(t, err)
	<-callbackCalled
}

func TestRun_GracefulTerminationDelay(t *testing.T) {
	t.Parallel()

	m := provider.New(new(containerMock))

	p := new(providerMock)
	setupProviderMock(p, "", nil, nil, nil, nil)
	m.Register(p)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	start := time.Now()
	err := m.Run(ctx, provider.WithTerminationDelay(50*time.Millisecond))
	elapsed := time.Since(start)

	require.NoError(t, err)
	assert.GreaterOrEqual(t, elapsed, 50*time.Millisecond)
}

func TestRun_TerminationDeadlineExceeded(t *testing.T) {
	t.Parallel()

	m := provider.New(new(containerMock))

	p := new(providerMock)
	p.On("Register", mock.Anything, mock.Anything).Return(nil)
	p.On("Boot", mock.Anything, mock.Anything).Return(nil)
	p.On("Terminate", mock.Anything).Run(func(args mock.Arguments) {
		// Block longer than the deadline
		time.Sleep(500 * time.Millisecond)
	}).Return(nil)
	m.Register(p)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := m.Run(ctx,
		provider.WithTerminationDelay(0),
		provider.WithTerminationDeadline(10*time.Millisecond),
	)
	assert.ErrorIs(t, err, context.DeadlineExceeded)
}
