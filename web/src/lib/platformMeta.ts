export interface FieldDef {
  key: string;
  labelKey: string;
  required?: boolean;
  type?: 'text' | 'password' | 'number' | 'boolean' | 'select';
  placeholder?: string;
  hintKey?: string;
  group?: 'basic' | 'advanced';
  options?: string[];
  showWhen?: Record<string, string[]>;
}

export interface PlatformMeta {
  label: string;
  fields: FieldDef[];
}

export const platformMeta: Record<string, PlatformMeta> = {
  wecom: {
    label: 'WeChat Work',
    fields: [
      { key: 'corp_id', labelKey: 'fields.corpId', required: true },
      { key: 'corp_secret', labelKey: 'fields.corpSecret', required: true, type: 'password' },
      { key: 'agent_id', labelKey: 'fields.agentId', required: true, placeholder: '1000002' },
      { key: 'callback_token', labelKey: 'fields.callbackToken', required: true },
      { key: 'callback_aes_key', labelKey: 'fields.callbackAesKey', required: true, hintKey: 'fields.callbackAesKeyHint' },
      { key: 'port', labelKey: 'fields.port', required: true, placeholder: '8081' },
      { key: 'callback_path', labelKey: 'fields.callbackPath', placeholder: '/wecom/callback', group: 'advanced' },
      { key: 'api_base_url', labelKey: 'fields.apiBaseUrl', placeholder: 'https://qyapi.weixin.qq.com', group: 'advanced' },
      { key: 'allow_from', labelKey: 'fields.allowFrom', placeholder: '* (all)', group: 'advanced' },
    ],
  },
};
