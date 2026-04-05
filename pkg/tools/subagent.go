package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/sipeed/picoclaw/pkg/fileutil"
	"github.com/sipeed/picoclaw/pkg/providers"
)

// SubTurnSpawner is an interface for spawning sub-turns.
// This avoids circular dependency between tools and agent packages.
type SubTurnSpawner interface {
	SpawnSubTurn(ctx context.Context, cfg SubTurnConfig) (*ToolResult, error)
}

// SubTurnConfig holds configuration for spawning a sub-turn.
type SubTurnConfig struct {
	Model              string
	Tools              []Tool
	SystemPrompt       string
	MaxTokens          int
	Temperature        float64
	Async              bool          // true for async (spawn), false for sync (subagent)
	Critical           bool          // continue running after parent finishes gracefully
	Timeout            time.Duration // 0 = use default (5 minutes)
	MaxContextRunes    int           // 0 = auto, -1 = no limit, >0 = explicit limit
	ActualSystemPrompt string
	InitialMessages    []providers.Message
	InitialTokenBudget *atomic.Int64 // Shared token budget for team members; nil if no budget
}

type TaskTeammate struct {
	ID             string   `json:"id"`
	Name           string   `json:"name,omitempty"`
	Role           string   `json:"role,omitempty"`
	AgentID        string   `json:"agent_id,omitempty"`
	Model          string   `json:"model,omitempty"`
	MemoryScope    string   `json:"memory_scope,omitempty"`
	ApprovalPolicy string   `json:"approval_policy,omitempty"`
	WorkspaceScope []string `json:"workspace_scope,omitempty"`
	Toolset        []string `json:"toolset,omitempty"`
}

type SpawnRequest struct {
	Task                string
	Label               string
	AgentID             string
	TeammateID          string
	RequesterAgentID    string
	RequesterTeammateID string
	OriginChannel       string
	OriginChatID        string
}

type SubagentTask struct {
	ID                  string   `json:"id"`
	Kind                string   `json:"kind"`
	Task                string   `json:"task"`
	Label               string   `json:"label,omitempty"`
	AgentID             string   `json:"agent_id,omitempty"`
	TeammateID          string   `json:"teammate_id,omitempty"`
	RequesterAgentID    string   `json:"requester_agent_id,omitempty"`
	RequesterTeammateID string   `json:"requester_teammate_id,omitempty"`
	OriginChannel       string   `json:"origin_channel,omitempty"`
	OriginChatID        string   `json:"origin_chat_id,omitempty"`
	ApprovalPolicy      string   `json:"approval_policy,omitempty"`
	ApprovedBy          string   `json:"approved_by,omitempty"`
	ApprovedAt          int64    `json:"approved_at,omitempty"`
	RejectedBy          string   `json:"rejected_by,omitempty"`
	RejectedAt          int64    `json:"rejected_at,omitempty"`
	ReviewNote          string   `json:"review_note,omitempty"`
	Status              string   `json:"status"`
	Result              string   `json:"result,omitempty"`
	MemoryScope         string   `json:"memory_scope,omitempty"`
	WorkspaceScope      []string `json:"workspace_scope,omitempty"`
	Created             int64    `json:"created"`
	Started             int64    `json:"started,omitempty"`
	Completed           int64    `json:"completed,omitempty"`
}

type SpawnSubTurnFunc func(
	ctx context.Context,
	task, label, agentID, teammateID string,
	tools *ToolRegistry,
	maxTokens int,
	temperature float64,
	hasMaxTokens, hasTemperature bool,
) (*ToolResult, error)

type TeammateResolver func(teammateID string) (TaskTeammate, bool)

type SubagentManager struct {
	tasks           map[string]*SubagentTask
	cancels         map[string]context.CancelFunc
	approvals       map[string]chan bool
	mu              sync.RWMutex
	provider        providers.LLMProvider
	defaultModel    string
	workspace       string
	stateFile       string
	legacyStateFile string
	tools           *ToolRegistry
	maxIterations   int
	maxTokens       int
	temperature     float64
	hasMaxTokens    bool
	hasTemperature  bool
	nextID          int
	spawner         SpawnSubTurnFunc
	teammates       TeammateResolver

	// mediaResolver resolves media:// refs in tool-loop messages before
	// each LLM call in the legacy RunToolLoop fallback path.
	// This lets subagents reuse the same media handling behavior as the
	// main agent loop without importing pkg/agent and creating a cycle.
	mediaResolver func([]providers.Message) []providers.Message
}

type subagentTaskStore struct {
	Version int            `json:"version"`
	NextID  int            `json:"next_id"`
	Tasks   []SubagentTask `json:"tasks"`
}

const subagentTaskStoreVersion = 1

func NewSubagentManager(
	provider providers.LLMProvider,
	defaultModel, workspace string,
	agentIDs ...string,
) *SubagentManager {
	agentID := "default"
	if len(agentIDs) > 0 && strings.TrimSpace(agentIDs[0]) != "" {
		agentID = strings.TrimSpace(agentIDs[0])
	}
	sm := &SubagentManager{
		tasks:           make(map[string]*SubagentTask),
		cancels:         make(map[string]context.CancelFunc),
		approvals:       make(map[string]chan bool),
		provider:        provider,
		defaultModel:    defaultModel,
		workspace:       workspace,
		stateFile:       filepath.Join(workspace, "state", "subagents", agentID, "tasks.json"),
		legacyStateFile: filepath.Join(workspace, "state", "subagents", "tasks.json"),
		tools:           NewToolRegistry(),
		maxIterations:   10,
		nextID:          1,
	}
	if err := sm.loadPersistedTasks(); err != nil {
		log.Printf("[WARN] subagent: failed to load persisted tasks: %v", err)
	}
	return sm
}

func (sm *SubagentManager) SetSpawner(spawner SpawnSubTurnFunc) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	sm.spawner = spawner
}

func (sm *SubagentManager) SetTeammateResolver(resolver TeammateResolver) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	sm.teammates = resolver
}

// SetMediaResolver injects a message preprocessor that resolves media:// refs
// into LLM-ready content before each tool-loop iteration.
// This is only used by the legacy RunToolLoop fallback path.
func (sm *SubagentManager) SetMediaResolver(
	resolver func([]providers.Message) []providers.Message,
) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	sm.mediaResolver = resolver
}

// SetLLMOptions sets max tokens and temperature for subagent LLM calls.
func (sm *SubagentManager) SetLLMOptions(maxTokens int, temperature float64) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	sm.maxTokens = maxTokens
	sm.hasMaxTokens = true
	sm.temperature = temperature
	sm.hasTemperature = true
}

// SetTools sets the tool registry for subagent execution.
// If not set, subagent will have access to the provided tools.
func (sm *SubagentManager) SetTools(tools *ToolRegistry) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	sm.tools = tools
}

// RegisterTool registers a tool for subagent execution.
func (sm *SubagentManager) RegisterTool(tool Tool) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	sm.tools.Register(tool)
}

func (sm *SubagentManager) Spawn(ctx context.Context, req SpawnRequest, callback AsyncCallback) (SubagentTask, error) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	taskID := fmt.Sprintf("subagent-%d", sm.nextID)
	sm.nextID++

	resolvedAgentID := req.AgentID
	var teammate TaskTeammate
	if strings.TrimSpace(req.TeammateID) != "" && sm.teammates != nil {
		var ok bool
		teammate, ok = sm.teammates(req.TeammateID)
		if !ok {
			return SubagentTask{}, fmt.Errorf("unknown teammate %q", req.TeammateID)
		}
		if resolvedAgentID == "" {
			resolvedAgentID = teammate.AgentID
		}
	}

	subagentTask := &SubagentTask{
		ID:                  taskID,
		Kind:                "delegation",
		Task:                req.Task,
		Label:               req.Label,
		AgentID:             resolvedAgentID,
		TeammateID:          req.TeammateID,
		RequesterAgentID:    req.RequesterAgentID,
		RequesterTeammateID: req.RequesterTeammateID,
		OriginChannel:       req.OriginChannel,
		OriginChatID:        req.OriginChatID,
		ApprovalPolicy:      teammate.ApprovalPolicy,
		Status:              "queued",
		MemoryScope:         teammate.MemoryScope,
		WorkspaceScope:      append([]string(nil), teammate.WorkspaceScope...),
		Created:             time.Now().UnixMilli(),
	}
	if requiresTaskApproval(teammate.ApprovalPolicy) {
		subagentTask.Status = "awaiting_approval"
		sm.approvals[taskID] = make(chan bool, 1)
	}
	sm.tasks[taskID] = subagentTask
	taskCtx, cancel := context.WithCancel(ctx)
	sm.cancels[taskID] = cancel
	if err := sm.persistLocked(); err != nil {
		delete(sm.tasks, taskID)
		delete(sm.cancels, taskID)
		delete(sm.approvals, taskID)
		cancel()
		return SubagentTask{}, fmt.Errorf("persist tracked task: %w", err)
	}

	// Start task in background with context cancellation support
	go sm.runTask(taskCtx, subagentTask, callback)

	return *subagentTask, nil
}

func (sm *SubagentManager) runTask(
	ctx context.Context,
	task *SubagentTask,
	callback AsyncCallback,
) {
	sm.mu.RLock()
	approvalWaiter := sm.approvals[task.ID]
	sm.mu.RUnlock()
	if approvalWaiter != nil {
		select {
		case approved := <-approvalWaiter:
			if !approved {
				sm.mu.Lock()
				delete(sm.approvals, task.ID)
				delete(sm.cancels, task.ID)
				if err := sm.persistLocked(); err != nil {
					log.Printf("[WARN] subagent: failed to persist rejected task %s: %v", task.ID, err)
				}
				sm.mu.Unlock()
				return
			}
		case <-ctx.Done():
			sm.mu.Lock()
			if !isSubagentTaskTerminal(task.Status) {
				task.Status = "canceled"
				task.Result = "Task canceled before approval"
				task.Completed = time.Now().UnixMilli()
			}
			delete(sm.approvals, task.ID)
			delete(sm.cancels, task.ID)
			if err := sm.persistLocked(); err != nil {
				log.Printf("[WARN] subagent: failed to persist canceled task %s: %v", task.ID, err)
			}
			sm.mu.Unlock()
			return
		}
	}

	// Check if context is already canceled before marking the task as running.
	select {
	case <-ctx.Done():
		sm.mu.Lock()
		if !isSubagentTaskTerminal(task.Status) {
			task.Status = "canceled"
			task.Result = "Task canceled before execution"
			task.Completed = time.Now().UnixMilli()
		}
		delete(sm.approvals, task.ID)
		delete(sm.cancels, task.ID)
		if err := sm.persistLocked(); err != nil {
			log.Printf("[WARN] subagent: failed to persist canceled task %s: %v", task.ID, err)
		}
		sm.mu.Unlock()
		return
	default:
	}

	sm.mu.Lock()
	if task.Status == "canceled" || task.Status == "denied" {
		delete(sm.approvals, task.ID)
		delete(sm.cancels, task.ID)
		if err := sm.persistLocked(); err != nil {
			log.Printf("[WARN] subagent: failed to persist pre-canceled task %s: %v", task.ID, err)
		}
		sm.mu.Unlock()
		return
	}
	task.Status = "running"
	task.Started = time.Now().UnixMilli()
	if err := sm.persistLocked(); err != nil {
		log.Printf("[WARN] subagent: failed to persist running task %s: %v", task.ID, err)
	}
	sm.mu.Unlock()
	// TODO(eventbus): once subagents are modeled as child turns inside
	// pkg/agent, emit SubTurnEnd and SubTurnResultDelivered from the parent
	// AgentLoop instead of this legacy manager.

	sm.mu.RLock()
	spawner := sm.spawner
	tools := sm.tools
	maxIter := sm.maxIterations
	maxTokens := sm.maxTokens
	temperature := sm.temperature
	hasMaxTokens := sm.hasMaxTokens
	hasTemperature := sm.hasTemperature
	mediaResolver := sm.mediaResolver
	sm.mu.RUnlock()

	var result *ToolResult
	var err error

	if spawner != nil {
		result, err = spawner(
			ctx,
			task.Task,
			task.Label,
			task.AgentID,
			task.TeammateID,
			tools,
			maxTokens,
			temperature,
			hasMaxTokens,
			hasTemperature,
		)
	} else {
		// Fallback to legacy RunToolLoop
		systemPrompt := `You are a subagent. Complete the given task independently and report the result.
You have access to tools - use them as needed to complete your task.
After completing the task, provide a clear summary of what was done.`

		messages := []providers.Message{
			{Role: "system", Content: systemPrompt},
			{Role: "user", Content: task.Task},
		}

		var llmOptions map[string]any
		if hasMaxTokens || hasTemperature {
			llmOptions = map[string]any{}
			if hasMaxTokens {
				llmOptions["max_tokens"] = maxTokens
			}
			if hasTemperature {
				llmOptions["temperature"] = temperature
			}
		}

		var loopResult *ToolLoopResult
		loopResult, err = RunToolLoop(ctx, ToolLoopConfig{
			Provider:      sm.provider,
			Model:         sm.defaultModel,
			Tools:         tools,
			MaxIterations: maxIter,
			LLMOptions:    llmOptions,
			MediaResolver: mediaResolver,
		}, messages, task.OriginChannel, task.OriginChatID)

		if err == nil {
			result = &ToolResult{
				ForLLM: fmt.Sprintf(
					"Subagent '%s' completed (iterations: %d): %s",
					task.Label,
					loopResult.Iterations,
					loopResult.Content,
				),
				ForUser: loopResult.Content,
				Silent:  false,
				IsError: false,
				Async:   false,
			}
		}
	}

	sm.mu.Lock()
	defer func() {
		delete(sm.approvals, task.ID)
		delete(sm.cancels, task.ID)
		if err := sm.persistLocked(); err != nil {
			log.Printf("[WARN] subagent: failed to persist final task state %s: %v", task.ID, err)
		}
		sm.mu.Unlock()
		// Call callback if provided and result is set
		if callback != nil && result != nil {
			callback(ctx, result)
		}
	}()

	if err != nil {
		task.Status = "failed"
		task.Result = fmt.Sprintf("Error: %v", err)
		task.Completed = time.Now().UnixMilli()
		// Check if it was canceled
		if ctx.Err() != nil {
			task.Status = "canceled"
			task.Result = "Task canceled during execution"
			task.Completed = time.Now().UnixMilli()
		}
		result = &ToolResult{
			ForLLM:  task.Result,
			ForUser: "",
			Silent:  false,
			IsError: true,
			Async:   false,
			Err:     err,
		}
	} else {
		task.Status = "completed"
		task.Result = result.ForLLM
		task.Completed = time.Now().UnixMilli()
	}
}

func (sm *SubagentManager) CancelTask(taskID string) (SubagentTask, error) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	task, ok := sm.tasks[taskID]
	if !ok {
		return SubagentTask{}, fmt.Errorf("task %q not found", taskID)
	}

	switch task.Status {
	case "awaiting_approval":
		task.Status = "canceled"
		task.Result = "Task canceled before approval"
		task.Completed = time.Now().UnixMilli()
	case "queued":
		task.Status = "canceled"
		task.Result = "Task canceled before execution"
		task.Completed = time.Now().UnixMilli()
	case "running":
		task.Status = "canceling"
		if task.Result == "" {
			task.Result = "Cancellation requested"
		}
	case "canceling":
		return *task, nil
	default:
		return SubagentTask{}, fmt.Errorf("task %q is not cancelable", taskID)
	}

	cancel := sm.cancels[taskID]
	approval := sm.approvals[taskID]
	if task.Status == "canceled" {
		if approval != nil {
			select {
			case approval <- false:
			default:
			}
		}
		delete(sm.approvals, taskID)
		delete(sm.cancels, taskID)
	}
	if err := sm.persistLocked(); err != nil {
		return SubagentTask{}, fmt.Errorf("persist canceled task: %w", err)
	}
	if cancel != nil {
		cancel()
	}
	return *task, nil
}

func (sm *SubagentManager) ApproveTask(taskID, actor, note string) (SubagentTask, error) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	task, ok := sm.tasks[taskID]
	if !ok {
		return SubagentTask{}, fmt.Errorf("task %q not found", taskID)
	}
	if task.Status != "awaiting_approval" {
		return SubagentTask{}, fmt.Errorf("task %q is not awaiting approval", taskID)
	}

	task.Status = "queued"
	task.ApprovedBy = strings.TrimSpace(actor)
	if task.ApprovedBy == "" {
		task.ApprovedBy = "launcher"
	}
	task.ApprovedAt = time.Now().UnixMilli()
	task.ReviewNote = strings.TrimSpace(note)
	waiter := sm.approvals[taskID]
	if err := sm.persistLocked(); err != nil {
		return SubagentTask{}, fmt.Errorf("persist approved task: %w", err)
	}
	if waiter != nil {
		select {
		case waiter <- true:
		default:
		}
	}
	return *task, nil
}

func (sm *SubagentManager) RejectTask(taskID, actor, note string) (SubagentTask, error) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	task, ok := sm.tasks[taskID]
	if !ok {
		return SubagentTask{}, fmt.Errorf("task %q not found", taskID)
	}
	if task.Status != "awaiting_approval" {
		return SubagentTask{}, fmt.Errorf("task %q is not awaiting approval", taskID)
	}

	task.Status = "denied"
	task.RejectedBy = strings.TrimSpace(actor)
	if task.RejectedBy == "" {
		task.RejectedBy = "launcher"
	}
	task.RejectedAt = time.Now().UnixMilli()
	task.Completed = task.RejectedAt
	task.ReviewNote = strings.TrimSpace(note)
	if task.ReviewNote == "" {
		task.ReviewNote = "Task denied during review"
	}
	task.Result = task.ReviewNote
	waiter := sm.approvals[taskID]
	if err := sm.persistLocked(); err != nil {
		return SubagentTask{}, fmt.Errorf("persist rejected task: %w", err)
	}
	if waiter != nil {
		select {
		case waiter <- false:
		default:
		}
	}
	return *task, nil
}

func (sm *SubagentManager) SupportsTrackedSpawn() bool {
	if sm == nil {
		return false
	}
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	return sm.spawner != nil
}

func (sm *SubagentManager) MarshalTask(task SubagentTask) string {
	data, err := json.Marshal(task)
	if err != nil {
		return fmt.Sprintf(`{"id":%q,"status":%q}`, task.ID, task.Status)
	}
	return string(data)
}

func (sm *SubagentManager) GetTask(taskID string) (*SubagentTask, bool) {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	task, ok := sm.tasks[taskID]
	return task, ok
}

// GetTaskCopy returns a copy of the task with the given ID, taken under the
// read lock, so the caller receives a consistent snapshot with no data race.
func (sm *SubagentManager) GetTaskCopy(taskID string) (SubagentTask, bool) {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	task, ok := sm.tasks[taskID]
	if !ok {
		return SubagentTask{}, false
	}
	return *task, true
}

func (sm *SubagentManager) ListTasks() []*SubagentTask {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	tasks := make([]*SubagentTask, 0, len(sm.tasks))
	for _, task := range sm.tasks {
		tasks = append(tasks, task)
	}
	return tasks
}

// ListTaskCopies returns value copies of all tasks, taken under the read lock,
// so callers receive consistent snapshots with no data race.
func (sm *SubagentManager) ListTaskCopies() []SubagentTask {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	copies := make([]SubagentTask, 0, len(sm.tasks))
	for _, task := range sm.tasks {
		copies = append(copies, *task)
	}
	return copies
}

func (sm *SubagentManager) loadPersistedTasks() error {
	if sm == nil || strings.TrimSpace(sm.stateFile) == "" {
		return nil
	}

	storePath := sm.stateFile
	data, err := os.ReadFile(storePath)
	if err != nil && os.IsNotExist(err) && strings.TrimSpace(sm.legacyStateFile) != "" {
		storePath = sm.legacyStateFile
		data, err = os.ReadFile(storePath)
	}
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	var store subagentTaskStore
	if err := json.Unmarshal(data, &store); err != nil {
		return err
	}
	if store.Version != 0 && store.Version != subagentTaskStoreVersion {
		return fmt.Errorf(
			"unsupported subagent task store version: got %d, want %d",
			store.Version,
			subagentTaskStoreVersion,
		)
	}

	now := time.Now().UnixMilli()
	needsRewrite := false
	loadedFromLegacy := storePath != sm.stateFile
	maxID := 0
	for i := range store.Tasks {
		task := store.Tasks[i]
		switch strings.ToLower(strings.TrimSpace(task.Status)) {
		case "awaiting_approval", "queued", "running", "canceling":
			task.Status = "failed"
			if strings.TrimSpace(task.Result) == "" {
				task.Result = "Task interrupted before completion during restart"
			}
			if task.Completed == 0 {
				task.Completed = now
			}
			needsRewrite = true
		}
		sm.tasks[task.ID] = &task
		if parsedID := parseSubagentTaskNumericID(task.ID); parsedID > maxID {
			maxID = parsedID
		}
	}

	if store.NextID > maxID {
		sm.nextID = store.NextID
	} else if maxID > 0 {
		sm.nextID = maxID + 1
	}

	if needsRewrite || loadedFromLegacy {
		if err := sm.persistLocked(); err != nil {
			return err
		}
		if loadedFromLegacy {
			_ = os.Remove(storePath)
		}
	}
	return nil
}

func (sm *SubagentManager) persistLocked() error {
	if sm == nil || strings.TrimSpace(sm.stateFile) == "" {
		return nil
	}

	tasks := make([]SubagentTask, 0, len(sm.tasks))
	for _, task := range sm.tasks {
		tasks = append(tasks, *task)
	}
	slices.SortFunc(tasks, func(a, b SubagentTask) int {
		if a.Created != b.Created {
			if a.Created < b.Created {
				return -1
			}
			return 1
		}
		switch {
		case a.ID < b.ID:
			return -1
		case a.ID > b.ID:
			return 1
		default:
			return 0
		}
	})

	payload, err := json.MarshalIndent(subagentTaskStore{
		Version: subagentTaskStoreVersion,
		NextID:  sm.nextID,
		Tasks:   tasks,
	}, "", "  ")
	if err != nil {
		return err
	}
	return fileutil.WriteFileAtomic(sm.stateFile, payload, 0o600)
}

func isSubagentTaskTerminal(status string) bool {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "completed", "failed", "canceled", "denied":
		return true
	default:
		return false
	}
}

func parseSubagentTaskNumericID(taskID string) int {
	const prefix = "subagent-"
	if !strings.HasPrefix(taskID, prefix) {
		return 0
	}
	id, err := strconv.Atoi(strings.TrimPrefix(taskID, prefix))
	if err != nil || id < 0 {
		return 0
	}
	return id
}

func requiresTaskApproval(policy string) bool {
	switch strings.TrimSpace(policy) {
	case "advice_only", "confirm_write", "confirm_exec":
		return true
	default:
		return false
	}
}

// SubagentTool executes a subagent task synchronously and returns the result.
// It directly calls SubTurnSpawner with Async=false for synchronous execution.
type SubagentTool struct {
	spawner      SubTurnSpawner
	defaultModel string
	maxTokens    int
	temperature  float64
}

func NewSubagentTool(manager *SubagentManager) *SubagentTool {
	if manager == nil {
		return &SubagentTool{}
	}
	return &SubagentTool{
		defaultModel: manager.defaultModel,
		maxTokens:    manager.maxTokens,
		temperature:  manager.temperature,
	}
}

// SetSpawner sets the SubTurnSpawner for direct sub-turn execution.
func (t *SubagentTool) SetSpawner(spawner SubTurnSpawner) {
	t.spawner = spawner
}

func (t *SubagentTool) Name() string {
	return "subagent"
}

func (t *SubagentTool) Description() string {
	return "Execute a subagent task synchronously and return the result. Use this for delegating specific tasks to an independent agent instance. Returns execution summary to user and full details to LLM."
}

func (t *SubagentTool) Parameters() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"task": map[string]any{
				"type":        "string",
				"description": "The task for subagent to complete",
			},
			"label": map[string]any{
				"type":        "string",
				"description": "Optional short label for the task (for display)",
			},
		},
		"required": []string{"task"},
	}
}

func (t *SubagentTool) Execute(ctx context.Context, args map[string]any) *ToolResult {
	task, ok := args["task"].(string)
	if !ok {
		return ErrorResult("task is required").WithError(fmt.Errorf("task parameter is required"))
	}

	label, _ := args["label"].(string)

	// Build system prompt for subagent
	systemPrompt := fmt.Sprintf(
		`You are a subagent. Complete the given task independently and provide a clear, concise result.

Task: %s`,
		task,
	)

	if label != "" {
		systemPrompt = fmt.Sprintf(
			`You are a subagent labeled "%s". Complete the given task independently and provide a clear, concise result.

Task: %s`,
			label,
			task,
		)
	}

	// Use spawner if available (direct SpawnSubTurn call)
	if t.spawner != nil {
		result, err := t.spawner.SpawnSubTurn(ctx, SubTurnConfig{
			Model:        t.defaultModel,
			Tools:        nil, // Will inherit from parent via context
			SystemPrompt: systemPrompt,
			MaxTokens:    t.maxTokens,
			Temperature:  t.temperature,
			Async:        false, // Synchronous execution
		})
		if err != nil {
			return ErrorResult(fmt.Sprintf("Subagent execution failed: %v", err)).WithError(err)
		}

		// Format result for display
		userContent := result.ForLLM
		if result.ForUser != "" {
			userContent = result.ForUser
		}
		maxUserLen := 500
		if len(userContent) > maxUserLen {
			userContent = userContent[:maxUserLen] + "..."
		}

		labelStr := label
		if labelStr == "" {
			labelStr = "(unnamed)"
		}
		llmContent := fmt.Sprintf("Subagent task completed:\nLabel: %s\nResult: %s",
			labelStr, result.ForLLM)

		return &ToolResult{
			ForLLM:  llmContent,
			ForUser: userContent,
			Silent:  false,
			IsError: result.IsError,
			Async:   false,
		}
	}

	// Fallback: spawner not configured
	return ErrorResult("Subagent manager not configured").WithError(fmt.Errorf("spawner not set"))
}
