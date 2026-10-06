#include "deps/include/v8-inspector.h"

#include "_cgo_export.h"
#include "context-macros.h"
#include "inspector.h"

using namespace v8;
using namespace v8_inspector;

class InspectorClient : public V8InspectorClient {
  uintptr_t _cgoHandle;

 public:
  InspectorClient(uintptr_t cgoHandle) { _cgoHandle = cgoHandle; }
  void consoleAPIMessage(int contextGroupId,
                         v8::Isolate::MessageErrorLevel level,
                         const StringView& message,
                         const StringView& url,
                         unsigned lineNumber,
                         unsigned columnNumber,
                         V8StackTrace*) override;
};

StringViewData ConvertStringView(const StringView& view) {
  StringViewData msg;
  msg.is8bit = view.is8Bit();

  msg.data =
      view.is8Bit() ? (void*)view.characters8() : (void*)view.characters16();
  msg.length = view.length();
  return msg;
}

void InspectorClient::consoleAPIMessage(int contextGroupId,
                                        v8::Isolate::MessageErrorLevel level,
                                        const StringView& message,
                                        const StringView& url,
                                        unsigned lineNumber,
                                        unsigned columnNumber,
                                        V8StackTrace*) {
  goHandleConsoleAPIMessageCallback(
      _cgoHandle, contextGroupId, level, ConvertStringView(message),
      ConvertStringView(url), lineNumber, columnNumber);
}

extern "C" {

v8Inspector* CreateInspector(v8Isolate* iso, v8InspectorClient* client) {
  v8Inspector* inspector = V8Inspector::create(iso, client).release();
  return inspector;
}

void DeleteInspector(v8Inspector* inspector) {
  delete inspector;
}

v8InspectorClient* NewInspectorClient(uintptr_t cgoHandle) {
  return new InspectorClient(cgoHandle);
}

void InspectorContextCreated(v8Inspector* inspector, ContextPtr context) {
  LOCAL_CONTEXT(context);
  int groupId = 1;
  V8ContextInfo info = V8ContextInfo(local_ctx, groupId, StringView());
  inspector->contextCreated(info);
}

void InspectorContextDestroyed(v8Inspector* inspector, ContextPtr context) {
  LOCAL_CONTEXT(context);
  inspector->contextDestroyed(local_ctx);
}

void DeleteInspectorClient(v8InspectorClient* client) {
  delete client;
}
}
