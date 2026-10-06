#include "deps/include/v8-context.h"
#include "deps/include/v8-initialization.h"
#include "deps/include/v8-locker.h"
#include "deps/include/v8-platform.h"
#include "deps/include/v8-profiler.h"
#include "deps/include/v8-promise.h"

#include "_cgo_export.h"
#include "context.h"
#include "isolate.h"
#include "libplatform/libplatform.h"

using namespace v8;

auto default_platform = platform::NewDefaultPlatform();
ArrayBuffer::Allocator* default_allocator;

class GoOutputStream : public OutputStream {
 public:
  explicit GoOutputStream(uintptr_t writerRef) : writerRef_(writerRef) {}

  int GetChunkSize() override { return 64 * 1024; }

  void EndOfStream() override {}

  WriteResult WriteAsciiChunk(char* data, int size) override {
    return goWriteHeapSnapshotChunk(writerRef_, data, size) ? kContinue
                                                            : kAbort;
  }

 private:
  uintptr_t writerRef_;
};

extern "C" {

struct IsolateState {
  bool heap_limit_reached = false;

  bool exception_messages = false;
};

#define ISOLATE_STATE_SLOT 1

#define ISOLATE_SCOPE(iso)           \
  Locker locker(iso);                \
  Isolate::Scope isolate_scope(iso); \
  HandleScope handle_scope(iso);

void Init() {
#ifdef _WIN32
  V8::InitializeExternalStartupData(".");
#endif
  V8::InitializePlatform(default_platform.get());
  V8::Initialize();

  default_allocator = ArrayBuffer::Allocator::NewDefaultAllocator();
  return;
}

size_t NearMemoryLimitCallback(void* data,
                               size_t current_heap_limit,
                               size_t initial_heap_limit) {
  auto iso = static_cast<Isolate*>(data);
  auto state = static_cast<IsolateState*>(iso->GetData(ISOLATE_STATE_SLOT));
  state->heap_limit_reached = true;
  iso->TerminateExecution();

  return current_heap_limit * 2;
}

IsolatePtr NewIsolate(IsolateConstraintsPtr constraints) {
  Isolate::CreateParams params;
  params.array_buffer_allocator = default_allocator;

  if (constraints != nullptr) {
    ResourceConstraints rc;
    rc.ConfigureDefaultsFromHeapSize(constraints->initial_heap_size_in_bytes,
                                     constraints->maximum_heap_size_in_bytes);
    params.constraints = rc;
  }

  Isolate* iso = Isolate::New(params);
  Locker locker(iso);
  Isolate::Scope isolate_scope(iso);
  HandleScope handle_scope(iso);

  iso->SetCaptureStackTraceForUncaughtExceptions(true);

  iso->SetData(ISOLATE_STATE_SLOT, new IsolateState);
  iso->AddNearHeapLimitCallback(NearMemoryLimitCallback, iso);

  iso->AutomaticallyRestoreInitialHeapLimit(0.5);

  m_ctx* ctx = new m_ctx;
  ctx->ptr.Reset(iso, Context::New(iso));
  ctx->iso = iso;
  iso->SetData(0, ctx);

  return iso;
}

void IsolatePerformMicrotaskCheckpoint(IsolatePtr iso) {
  ISOLATE_SCOPE(iso)
  iso->PerformMicrotaskCheckpoint();
}

void IsolateDispose(IsolatePtr iso) {
  if (iso == nullptr) {
    return;
  }
  auto ctx = static_cast<m_ctx*>(iso->GetData(0));
  ContextFree(ctx);
  auto state = static_cast<IsolateState*>(iso->GetData(ISOLATE_STATE_SLOT));

  iso->Dispose();
  delete state;
}

void IsolateSetExceptionMessages(IsolatePtr iso, int enabled) {
  auto state = static_cast<IsolateState*>(iso->GetData(ISOLATE_STATE_SLOT));
  state->exception_messages = enabled;
}

int IsolateExceptionMessages(IsolatePtr iso) {
  auto state = static_cast<IsolateState*>(iso->GetData(ISOLATE_STATE_SLOT));
  return state->exception_messages;
}

int IsolateTakeHeapLimitReached(IsolatePtr iso) {
  auto state = static_cast<IsolateState*>(iso->GetData(ISOLATE_STATE_SLOT));
  if (!state->heap_limit_reached) {
    return 0;
  }
  state->heap_limit_reached = false;
  return 1;
}

void IsolateTerminateExecution(IsolatePtr iso) {
  iso->TerminateExecution();
}

int IsolateIsExecutionTerminating(IsolatePtr iso) {
  return iso->IsExecutionTerminating();
}

void IsolateLowMemoryNotification(IsolatePtr iso) {
  ISOLATE_SCOPE(iso);
  iso->LowMemoryNotification();
}

int IsolateWriteHeapSnapshotChecked(IsolatePtr iso, uintptr_t writerRef) {
  if (iso == nullptr) {
    return 0;
  }

  ISOLATE_SCOPE(iso);

  const HeapSnapshot* snapshot = iso->GetHeapProfiler()->TakeHeapSnapshot();
  if (snapshot == nullptr) {
    return 0;
  }

  GoOutputStream stream(writerRef);
  snapshot->Serialize(&stream, HeapSnapshot::kJSON);
  const_cast<HeapSnapshot*>(snapshot)->Delete();
  return 1;
}

void IsolateWriteHeapSnapshot(IsolatePtr iso, uintptr_t writerRef) {
  IsolateWriteHeapSnapshotChecked(iso, writerRef);
}

static_assert(kPromiseRejectWithNoHandler == 0);
static_assert(kPromiseHandlerAddedAfterReject == 1);

static void PromiseRejectedCallback(PromiseRejectMessage message) {
  Isolate* iso = Isolate::GetCurrent();
  Local<Promise> promise = message.GetPromise();

  Local<Context> local_ctx;
  if (!promise->GetCreationContext(iso).ToLocal(&local_ctx)) {
    return;
  }

  int ctx_ref = local_ctx->GetEmbedderDataV2(ContextDataIndex::REF)
                    .As<Integer>()
                    ->Value();
  m_ctx* ctx = goContext(ctx_ref);
  if (ctx == nullptr) {
    return;
  }

  Local<Value> value = message.GetValue();
  goPromiseRejectedCallback(
      ctx_ref, message.GetEvent(), track_value(ctx, promise),
      value.IsEmpty() ? nullptr : track_value(ctx, value));
}

void IsolateSetPromiseRejectedCallback(IsolatePtr iso, bool enable) {
  ISOLATE_SCOPE(iso);
  iso->SetPromiseRejectCallback(enable ? PromiseRejectedCallback : nullptr);
}

IsolateHStatistics IsolationGetHeapStatistics(IsolatePtr iso) {
  if (iso == nullptr) {
    return IsolateHStatistics{0};
  }
  v8::HeapStatistics hs;
  iso->GetHeapStatistics(&hs);

  return IsolateHStatistics{hs.total_heap_size(),
                            hs.total_heap_size_executable(),
                            hs.total_physical_size(),
                            hs.total_available_size(),
                            hs.used_heap_size(),
                            hs.heap_size_limit(),
                            hs.malloced_memory(),
                            hs.external_memory(),
                            hs.peak_malloced_memory(),
                            hs.number_of_native_contexts(),
                            hs.number_of_detached_contexts()};
}
}
