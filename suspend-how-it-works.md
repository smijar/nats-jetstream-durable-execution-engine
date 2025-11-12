> can you add a detailed architecture.md that explains exactly how nats jetstream is used with code sections of processor or other that uses nats for the log-based workflow execution?

✦ Of course. That's an excellent idea to add a deeper dive into the core mechanism of the engine. I will add a new, detailed section to architecture.md that explains exactly how NATS JetStream
  is used, complete with code snippets from the relevant packages.

  Here is the explanation first, and then I will append it to the architecture.md file.

  ---

  Detailed NATS JetStream Integration: The Log-Based Engine

  The entire durable execution engine is built upon a log-based architecture, where NATS JetStream provides the durable, replayable log. This approach treats every action as an immutable event
   that is recorded first, ensuring the system can recover its state perfectly after any failure.

  Here’s a breakdown of how the key components in NATS JetStream are used.

  1. The Command Log (JetStream Stream)

  The primary log is a NATS JetStream stream named COMMANDS. It serves as the durable, ordered queue for all workflow invocation requests.

   * Purpose: To ensure that every request to start or resume a workflow is captured durably and will not be lost.
   * Configuration: The stream is configured in pkg/jetstream/client.go to be a work queue that retains messages until they are explicitly acknowledged by a consumer.

   1     // From: pkg/jetstream/client.go
   2
   3     // StreamConfig defines the configuration for the command stream
   4     var StreamConfig = &nats.StreamConfig{
   5     	Name:     "COMMANDS",
   6     	Subjects:  [string{"COMMANDS.*"},
   7     	Retention: nats.WorkQueuePolicy,// Message is removed once acknowledged
   8     	Storage:   nats.FileStorage,
   9     }

   * Publishing Commands: When a client invokes a workflow, it serializes a durable.Command protobuf message and publishes it to a subject that the stream is listening on (e.g.,
     COMMANDS.default).

   1     // From: pkg/jetstream/client.go
   2
   3     // PublishCommand publishes a command to the durable stream
   4     func (c *Client) PublishCommand(ctx context.Context, partitionKey string, data []byte) error {
   5     	subject := fmt.Sprintf"COMMANDS.%s", partitionKey)
   6     	_, err := c.js.Publish(ctx, subject, data)
   7     return err
   8     }

  2. The State Store (JetStream KV Bucket)

  The state of every single workflow execution is stored in a NATS JetStream Key-Value (KV) bucket named EXECUTION_STATE.

   * Purpose: To persist the ExecutionState protobuf message for each workflow. This message contains the workflow's status, its full journal of completed steps, and any workflow-scoped state
     (ctx.Get/Set).
   * Configuration: The KV bucket is also defined in pkg/jetstream/client.go. The key is the unique invocation_id of the workflow.

   1     // From: pkg/jetstream/client.go
   2
   3     // StateKVConfig defines the configuration for the execution state KV bucket
   4     var StateKVConfig = &nats.KeyValueConfig{
   5     	Bucket: "EXECUTION_STATE",
   6     	Storage: nats.FileStorage,
   7     }

   * Saving and Loading State: The processor interacts with this KV bucket before and after executing workflow logic.

    1     // From: pkg/execution/processor.go
    2
    3     // loadState loads execution state from KV bucket
    4     func (p *Processor) loadState(ctx context.Context, invocationID string) (*durable.ExecutionState, error) {
    5     	entry, err := p.kv.Get(ctx, invocationID)
    6     if err != nil {
    7     // ... handle not found ...
    8     	}
    9     var state durable.ExecutionState
   10     	proto.Unmarshal(entry.Value(), &state)
   11     return &state, nil
   12     }
   13
   14     // saveState saves execution state to KV bucket atomically
   15     func (p *Processor) saveState(ctx context.Context, state *durable.ExecutionState) error {
   16     	data, err := proto.Marshal(state)
   17     // ... handle error ...
   18     	_, err = p.kv.Put(ctx, state.InvocationId, data)
   19     return err
   20     }

  3. The Processor's Consumption and Ack/Nak Logic

  This is the most critical part of the engine, where the command log and state store come together. The processor consumes messages from the COMMANDS stream and uses the EXECUTION_STATE KV to
   decide what to do.

   * Purpose: To read a command, execute the corresponding workflow logic, and then explicitly acknowledge (Ack) or negatively acknowledge (Nak) the message to control its lifecycle in the
     stream.
   * Implementation: The core logic resides in the processMessage function in pkg/execution/processor.go.

    1     // From: pkg/execution/processor.go
    2
    3     // processMessage processes a single command message
    4     func (p *Processor) processMessage(ctx context.Context, msg natsjs.Msg) error {
    5     // ... unmarshal command ...
    6
    7     // Load the workflow's current state from the KV store
    8     	state, err := p.loadState(ctx, cmd.InvocationId)
    9     // ... handle error ...
   10
   11     // Check the status from the state store
   12     if state != nil {
   13     switch state.Status {
   14     case "completed", "cancelled":
   15     // If already done, just Ack the message to remove it from the stream
   16     			log.Prin"Execution already completed/cancelled: %s", cmd.InvocationId)
   17     return nil // This will lead to an Ack
   18     case "paused":
   19     // If paused, Nak the message to have it redelivered later
   20     			log.Prin"Execution paused: %s (will retry later)", cmd.InvocationId)
   21     return fmt.Errorf("execution paused: %s", cmd.InvocationId) // This will lead to a Nak
   22     		}
   23     	}
   24
   25     // ... create durable context and replay journal ...
   26
   27     // Execute the actual workflow handler
   28     if err := handler(durableContext); err != nil {
   29     // If the handler fails (e.g., waiting for an awakeable or a real error)
   30     // Nak the message so it gets retried.
   31     		log.Print"Handler execution failed: %v", err)
   32     		state.Status "failed"
   33     		p.saveState(ctx, state// Save the "failed" status
   34     return fmt.Errorf("handler execution failed: %w", err)
   35     	}
   36
   37     // If everything was successful:
   38     	state.Status ="completed"
   39     if err := p.saveState(ctx, state); err != nil {
   40     // If we can't save the final state, Nak to retry the whole step
   41     return fmt.Errorf("failed to save final state: %w", err)
   42     	}
   43
   44     	log.Printf"Execution completed successfully: %s", cmd.InvocationId)
   45     // A nil return here signals the message can be Ack'd and removed.
   46     return nil
   47     }

  This Ack/Nak logic is the engine that drives everything:
   * A successful run results in an Ack, completing the workflow.
   * A temporary failure or a "paused" state results in a Nak, ensuring the workflow will be retried.
   * A permanent cancellation results in an Ack, ensuring the workflow is never run again.

  ---

  I will now append this detailed section to the architecture.md file.
╭───────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────╮
│ ✓  ReadFile architecture.md                                                                                                                                                               │
╰───────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────╯
╭───────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────╮
│ ✓  WriteFile Writing to architecture.md                                                                                                                                                   │
│                                                                                                                                                                                           │
│    ... first 76 lines hidden ...                                                                                                                                                          │
│    263 +     ```                                                                                                                                                                          │
│    264 +                                                                                                                                                                                  │
│    265 + *   **Saving and Loading State:** The processor interacts with this KV bucket before and after executing workflow logic.                                                         │
│    266 +                                                                                                                                                                                  │
│    267 +     ```go                                                                                                                                                                        │
│    268 +     // From: pkg/execution/processor.go                                                                                                                                          │
│    269 +                                                                                                                                                                                  │
│    270 +     // loadState loads execution state from KV bucket                                                                                                                            │
│    271 +     func (p *Processor) loadState(ctx context.Context, invocationID string) (*durable.ExecutionState, error) {                                                                   │
│    272 +         entry, err := p.kv.Get(ctx, invocationID)                                                                                                                                │
│    273 +         if err != nil {                                                                                                                                                          │
│    274 +             // ... handle not found ...                                                                                                                                          │
│    275 +         }                                                                                                                                                                        │
│    276 +         var state durable.ExecutionState                                                                                                                                         │
│    277 +         proto.Unmarshal(entry.Value(), &state)                                                                                                                                   │
│    278 +         return &state, nil                                                                                                                                                       │
│    279 +     }                                                                                                                                                                            │
│    280 +                                                                                                                                                                                  │
│    281 +     // saveState saves execution state to KV bucket atomically                                                                                                                   │
│    282 +     func (p *Processor) saveState(ctx context.Context, state *durable.ExecutionState) error {                                                                                    │
│    283 +         data, err := proto.Marshal(state)                                                                                                                                        │
│    284 +         // ... handle error ...                                                                                                                                                  │
│    285 +         _, err = p.kv.Put(ctx, state.InvocationId, data)                                                                                                                         │
│    286 +         return err                                                                                                                                                               │
│    287 +     }                                                                                                                                                                            │
│    288 +     ```                                                                                                                                                                          │
│    289 +                                                                                                                                                                                  │
│    290 + ### 3. The Processor's Consumption and Ack/Nak Logic                                                                                                                             │
│    291 +                                                                                                                                                                                  │
│    292 + This is the most critical part of the engine, where the command log and state store come together. The processor consumes messages from the `COMMANDS` stream and uses the       │
│        `EXECUTION_STATE` KV to decide what to do.                                                                                                                                         │
│    293 +                                                                                                                                                                                  │
│    294 + *   **Purpose:** To read a command, execute the corresponding workflow logic, and then explicitly acknowledge (`Ack`) or negatively acknowledge (`Nak`) the message to control   │
│        its lifecycle in the stream.                                                                                                                                                       │
│    295 + *   **Implementation:** The core logic resides in the `processMessage` function in `pkg/execution/processor.go`.                                                                 │
│    296 +                                                                                                                                                                                  │
│    297 +     ```go                                                                                                                                                                        │
│    298 +     // From: pkg/execution/processor.go                                                                                                                                          │
│    299 +                                                                                                                                                                                  │
│    300 +     // processMessage processes a single command message                                                                                                                         │
│    301 +     func (p *Processor) processMessage(ctx context.Context, msg natsjs.Msg) error {                                                                                              │
│    302 +         // ... unmarshal command ...                                                                                                                                             │
│    303 +                                                                                                                                                                                  │
│    304 +         // Load the workflow's current state from the KV store                                                                                                                   │
│    305 +         state, err := p.loadState(ctx, cmd.InvocationId)                                                                                                                         │
│    306 +         // ... handle error ...                                                                                                                                                  │
│    307 +                                                                                                                                                                                  │
│    308 +         // Check the status from the state store                                                                                                                                 │
│    309 +         if state != nil {                                                                                                                                                        │
│    310 +             switch state.Status {                                                                                                                                                │
│    311 +             case "completed", "cancelled":                                                                                                                                       │
│    312 +                 // If already done, just Ack the message to remove it from the stream                                                                                            │
│    313 +                 log.Printf("Execution already completed/cancelled: %s", cmd.InvocationId)                                                                                        │
│    314 +                 return nil // This will lead to an Ack                                                                                                                           │
│    315 +             case "paused":                                                                                                                                                       │
│    316 +                 // If paused, Nak the message to have it redelivered later                                                                                                       │
│    317 +                 log.Printf("Execution paused: %s (will retry later)", cmd.InvocationId)                                                                                          │
│    318 +                 return fmt.Errorf("execution paused: %s", cmd.InvocationId) // This will lead to a Nak                                                                           │
│    319 +             }                                                                                                                                                                    │
│    320 +         }                                                                                                                                                                        │
│    321 +                                                                                                                                                                                  │
│    322 +         // ... create durable context and replay journal ...                                                                                                                     │
│    323 +                                                                                                                                                                                  │
│    324 +         // Execute the actual workflow handler                                                                                                                                   │
│    325 +         if err := handler(durableContext); err != nil {                                                                                                                          │
│    326 +             // If the handler fails (e.g., waiting for an awakeable or a real error)                                                                                             │
│    327 +             // Nak the message so it gets retried.                                                                                                                               │
│    328 +             log.Printf("Handler execution failed: %v", err)                                                                                                                      │
│    329 +             state.Status = "failed"                                                                                                                                              │
│    330 +             p.saveState(ctx, state) // Save the "failed" status                                                                                                                  │
│    331 +             return fmt.Errorf("handler execution failed: %w", err)                                                                                                               │
│    332 +         }                                                                                                                                                                        │
│    333 +                                                                                                                                                                                  │
│    334 +         // If everything was successful:                                                                                                                                         │
│    335 +         state.Status = "completed"                                                                                                                                               │
│    336 +         if err := p.saveState(ctx, state); err != nil {                                                                                                                          │
│    337 +             // If we can't save the final state, Nak to retry the whole step                                                                                                     │
│    338 +             return fmt.Errorf("failed to save final state: %w", err)                                                                                                             │
│    339 +         }                                                                                                                                                                        │
│    340 +                                                                                                                                                                                  │
│    341 +         log.Printf("Execution completed successfully: %s", cmd.InvocationId)                                                                                                     │
│    342 +         // A nil return here signals the message can be Ack'd and removed.                                                                                                       │
│    343 +         return nil                                                                                                                                                               │
│    344 +     }                                                                                                                                                                            │
│    345 +     ```                                                                                                                                                                          │
│    346 +                                                                                                                                                                                  │
│    347 + This `Ack`/`Nak` logic is the engine that drives everything:                                                                                                                     │
│    348 + *   A successful run results in an `Ack`, completing the workflow.                                                                                                               │
│    349 + *   A temporary failure or a "paused" state results in a `Nak`, ensuring the workflow will be retried.                                                                           │
│    350 + *   A permanent cancellation results in an `Ack`, ensuring the workflow is never run again.                                                                                      │
╰───────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────╯
✦ I have updated the architecture.md file with the detailed explanation of the NATS JetStream integration, including the code snippets and the clarification on how ctx.Awakeable works.




> var ExpenseApprovalWorkflow = durable.NewWorkflow("expense_approval",
      func(ctx *durable.Context, request ExpenseRequest) (string, error) {
          // ... log the request ...

          // Step 2: Create an awakeable and suspend the workflow.
          // The workflow will not proceed past this line until the API is called.
          awakeableID := fmt.Sprintf("approval-%s", uuid.New().String())
          log.Printf("⏳ Waiting for manager approval (ID: %s)", awakeableID)
          resultBytes, err := ctx.Awakeable(awakeableID)
          if err != nil {
              // This block runs if the awakeable was rejected via the API
              return "Rejected", err
          }

          // Step 3: The workflow resumes here after the awakeable is resolved.
          var approval ApprovalResult
          json.Unmarshal(resultBytes, &approval)
          if approval.Approved {
              return "✅ Approved!", nil
          }

          return "❌ Rejected.", nil
      })

  in this awakeable example in architecture.md , how can cpu execution stop at line

✦ That is an excellent, deep question. You've pinpointed the "magic" that makes this entire pattern work. The CPU execution doesn't "stop" in the way a thread blocks in traditional
  programming. Instead, the workflow aborts its current execution and relies on the engine to resume it later.

  Here is the step-by-step breakdown of what happens at that exact line:

  resultBytes, err := ctx.Awakeable(awakeableID)

   1. Inside `ctx.Awakeable`: The function first checks its internal state to see if the result for awakeableID has already been provided (e.g., if the API call happened before the workflow
      got to this step).

   2. The "Suspend" Path: In the normal case, the result is not there. The Awakeable function then does something very specific: it returns a special, internal error type called
      AwakeableSuspendError.

   3. Aborting the Handler: This special error propagates all the way up and is returned by the ExpenseApprovalWorkflow handler function itself. The Go code inside your workflow stops
      executing at this point. The json.Unmarshal line is not reached.

   4. The Processor Catches the Special Error: The processMessage function in the processor is what called your workflow handler. It receives this AwakeableSuspendError. Instead of treating it
       like a real failure, it recognizes it as a signal to suspend.

    1     // Simplified logic from pkg/execution/processor.go
    2
    3     // Execute handler
    4     if err := handler(durableContext); err != nil {
    5         // Check if this is our special suspend signal
    6         if _, ok := err.(*durableCtx.AwakeableSuspendError); ok {
    7             log.Printf("Workflow suspended waiting for awakeable...")
    8             // Return an error to Nak the message, ensuring it will be redelivered
    9             return fmt.Errorf("awaiting awakeable")
   10         }
   11         // ... handle real failures ...
   12     }

   5. Entering the Holding Pattern: Because the processor returns an error, the NATS message is negatively acknowledged (`Nak`). This tells NATS JetStream to redeliver the command later. The
      processor is now free to work on other workflows. The CPU is not blocked.

   6. Resumption: Later, when the external API call resolves the awakeable, it saves the result in the ExecutionState within the NATS KV store. The next time NATS redelivers the command, the
      processor starts the workflow from the beginning.

   7. The Magic of Replay: The processor replays the journal. When it gets to the ctx.Awakeable(awakeableID) line again, the Awakeable function checks its state, sees that the result is now
      available in the ExecutionState, and immediately returns the result instead of the special error.

   8. Execution Continues: The err is nil, so the workflow's execution continues past that line to the json.Unmarshal step, using the resultBytes that were just retrieved from the replayed
      state.

  In summary: The CPU execution doesn't "stop" at that line. The entire function aborts with a special error that the engine understands. The magic lies in the deterministic replay: when
  the workflow is attempted again with the resolved state, it replays instantly to the point where it was suspended and then continues with the new information.

> But, what if there are calls before the awakeable line, that get replayed on next run?

✦ That is the perfect follow-up question, and it gets to the absolute core of why this architecture is called a durable execution engine with deterministic replay.

  You are correct: on the next run
✕ [API Error: got status: UNAVAILABLE. {"error":{"code":503,"message":"The request timed out. Please try again.","status":"UNAVAILABLE"}}]


> But, what if there are calls before the awakeable line, that get replayed on next run?

✦ That is the most critical question in this entire architecture, and you've hit on the exact reason the journal is so important.

  The answer is: The calls before the Awakeable line are re-played, but they are not re-executed.

  This is the core principle of deterministic replay. Here’s a detailed breakdown of the sequence:

  The First Run (Execution until Suspension)

  Let's imagine a workflow with three steps:

    1 func MyWorkflow(ctx *durable.Context) error {
    2     // Step 1: A durable call
    3     ctx.DurableCall("ServiceA", "DoSomething", req1, resp1)
    4
    5     // Step 2: The awakeable that suspends the workflow
    6     resultBytes, err := ctx.Awakeable("my-awakeable")
    7     if err != nil { return err }
    8
    9     // Step 3: Another durable call after resumption
   10     ctx.DurableCall("ServiceB", "DoSomethingElse", req2, resp2)
   11
   12     return nil
   13 }

  Here's what happens the first time the processor runs this:

   1. Execute Step 1: The processor encounters DurableCall("ServiceA", ...).
       * It checks the journal. The journal is empty.
       * It makes a real network call to ServiceA.
       * ServiceA returns a result.
       * The processor records the result of Step 1 in the journal and saves the entire ExecutionState to the NATS KV store. The journal now looks like: [{step: 0, service: "ServiceA",
         result: "..."}].

   2. Execute Step 2: The processor encounters ctx.Awakeable("my-awakeable").
       * It checks the state and sees the awakeable has not been resolved.
       * It returns the special AwakeableSuspendError.
       * The workflow handler aborts.
       * The processor catches the special error and negatively acknowledges (`Nak`) the NATS message, putting the workflow into its holding pattern.

  At this point, the workflow is suspended, and the NATS KV store contains a journal with the successful result of Step 1.

  The Resume Run (Execution after Awakeable is Resolved)

  Sometime later, an external system calls the API to resolve "my-awakeable". The result is saved to the ExecutionState in the KV store. NATS then redelivers the command to the processor.

  Here's what happens now:

   1. Start from the Beginning: The processor starts executing the MyWorkflow function from the very first line.

   2. Replay Step 1: The processor encounters DurableCall("ServiceA", ...).
       * It checks the journal. It sees an entry for step 0.
       * Instead of making a network call, it immediately returns the result from the journal entry.
       * ServiceA is NOT called again. This is the "replay." It's an extremely fast, in-memory operation that brings the workflow's state back to where it was.

   3. Execute Step 2: The processor encounters ctx.Awakeable("my-awakeable").
       * It checks the state and sees the awakeable has now been resolved.
       * It returns the result that was provided by the external API call. The err is nil.
       * The workflow's execution continues past the Awakeable line.

   4. Execute Step 3: The processor encounters DurableCall("ServiceB", ...).
       * It checks the journal. There is no entry for step 1 (or whatever the current step number is).
       * It makes a real network call to ServiceB.
       * It records the result in the journal.

  Analogy: The Scientist's Lab Notebook

  Think of the processor as a scientist and the journal as their lab notebook.

   * Day 1: The scientist performs Experiment A and writes the result in the notebook. Then, they get to a step that says "Wait 24 hours." They stop for the day.
   * Day 2: The scientist comes back. Do they re-do Experiment A? No. They simply read the notebook to get the result of Experiment A, and then proceed with the next step.

  The journal ensures that any durable action is only ever executed once. On subsequent runs, its result is simply read from the log. This is what makes the entire system both durable and
  efficient.

 Using: 1 GEMINI.md file | 1 MCP server                                                                                                               accepting edits (shift + tab to toggle)