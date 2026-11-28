import { useI18n } from './index';
import { useListenerRuntimeMessages } from './listenerRuntime';

export function useOutboundConnectionMessages() {
  const common = useListenerRuntimeMessages();
  const zh = useI18n().locale.startsWith('zh');
  return {
    ...common,
    scope: zh
      ? '临时 Mihomo 客户端使用已保存的出站配置及其拨号依赖请求测试地址，无需应用配置。结果仅代表本服务器到测试目标的连接情况。'
      : 'A temporary Mihomo client tests the saved outbound settings and dialer dependencies. Applying the configuration is not required. Results describe this server’s connection to the test targets.',
    reasons: {
      ...common.reasons,
      disabled: zh ? '该出站已禁用。' : 'This outbound is disabled.',
      client_config_ready: zh ? '使用已保存的出站协议设置及拨号依赖。' : 'Using the saved outbound protocol settings and dialer dependencies.',
      client_start_failed: zh ? '测试客户端未能启动，请检查 Mihomo 可用性及已保存的出站配置。' : 'The test client could not start. Check Mihomo availability and the saved outbound settings.',
      proxy_request_passed: zh ? '通过该出站收到测试目标的预期响应。' : 'The test target returned the expected response through this outbound.',
      proxy_request_failed: zh ? '测试目标均未通过该出站返回预期响应，请检查服务器、凭证、TLS 和拨号依赖。' : 'Neither test target returned the expected response through this outbound. Check its server, credentials, TLS settings and dialer dependencies.',
    } as Record<string, string>,
  };
}
