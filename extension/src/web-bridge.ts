// web-bridge.ts —— 网页桥（注入授权服务页面 https://ns.example.com/*）
//
// 职责：把网页发起的「一键填充私信验证码」请求桥接给扩展后台：
//   网页 window.postMessage({type:'nsauth2-fill-pm', code, toUserId}, location.origin)
//   → 本脚本 chrome.runtime.sendMessage({type:'fill-pm', code, toUserId}) 转发后台
//   → 收到后台结果后 window.postMessage({type:'nsauth2-fill-pm-result', ok, message}, location.origin) 回传网页。
// 此桥只做转发，不依赖 serverUrl 等扩展配置。
(() => {
  const BRIDGE_IN = 'nsauth2-fill-pm'; // 网页 → 扩展
  const BRIDGE_OUT = 'nsauth2-fill-pm-result'; // 扩展 → 网页

  // 校验网页消息形状：{type:'nsauth2-fill-pm', code:string, toUserId:string}
  function isFillPmMessage(data: unknown): data is { type: string; code: string; toUserId: string } {
    if (!data || typeof data !== 'object') return false;
    const d = data as Record<string, unknown>;
    return d.type === BRIDGE_IN && typeof d.code === 'string' && typeof d.toUserId === 'string';
  }

  window.addEventListener('message', (event) => {
    // 只接受同窗口（本页面 SPA）发出的消息，防止其他网页伪造触发
    if (event.source !== window) return;
    if (!isFillPmMessage(event.data)) return;

    const { code, toUserId } = event.data;
    // 转发给后台 Service Worker：找/建 nodeseek 标签页并填充
    void chrome.runtime.sendMessage({ type: 'fill-pm', code, toUserId }).then((res) => {
      const r = (res ?? {}) as { ok?: boolean; message?: string };
      // 结果回传网页（targetOrigin 限定为本页源，避免验证码相关状态泄露给其他源）
      window.postMessage(
        { type: BRIDGE_OUT, ok: Boolean(r.ok), message: String(r.message ?? '') },
        location.origin
      );
    });
  });
})();
