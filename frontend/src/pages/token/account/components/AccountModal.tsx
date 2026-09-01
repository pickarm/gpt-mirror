import accountService, {AccountAddReq} from "@/api/services/accountService.ts";
import {
  Form,
  Input,
  Modal,
  Select,
  Space,
  Spin,
  Switch,
  Tag,
  Tooltip,
  Typography
} from "antd";
import {useEffect, useState} from "react";
import {useTranslation} from "react-i18next";
import Password from "antd/es/input/Password";
import {InfoCircleOutlined} from "@ant-design/icons";
import {useQuery} from "@tanstack/react-query";

export type AccountModalProps = {
  formValue: AccountAddReq;
  title: string;
  show: boolean;
  onOk: (values: AccountAddReq, setLoading: any) => void;
  onCancel: VoidFunction;
};

export function AccountModal({ title, show, formValue, onOk, onCancel }: AccountModalProps) {
  const [form1] = Form.useForm();
  const [loading, setLoading] = useState(false);
  const { t } = useTranslation();

  const { data, isLoading } = useQuery({
    queryKey: ['accounts', 'one-api-channel', formValue],
    queryFn: () => accountService.getOneApiChannelList(),
    enabled: show,
  });

  useEffect(() => {
    if (show) {
      form1.setFieldsValue(formValue)
    }
  }, [show, formValue, form1]);

  const onModalOk = () => {
    form1.validateFields().then((values) => {
      setLoading(true);
      onOk(values, setLoading);
    });
  };

  const credentialPlaceholder = formValue.hasCredential
    ? '留空则保留现有加密凭据；填写后更新该秘密字段'
    : '填写后保存到加密 credential store';
  const proxyPlaceholder = formValue.proxyDisplay
    ? `当前 ${formValue.proxyDisplay}；留空则保留`
    : '例如 socks5h://user:pass@127.0.0.1:1080';

  return (
    <Modal
      title={title}
      open={show}
      onOk={onModalOk}
      onCancel={() => {
        form1.resetFields();
        onCancel();
      }}
      okButtonProps={{
        loading,
      }}
      destroyOnClose={true}
    >
      <Form
        initialValues={formValue}
        form={form1}
        layout="vertical"
        preserve={false}
        autoComplete="off"
      >
        <Form.Item<AccountAddReq> name="id" hidden>
          <Input />
        </Form.Item>
        <Form.Item<AccountAddReq> name="accountType" hidden>
          <Input />
        </Form.Item>
        <Form.Item<AccountAddReq> label="Email" name="email" required>
          <Input placeholder={"仅作账户标记"} />
        </Form.Item>
        <Form.Item<AccountAddReq> label={t('token.password')} name="password">
          <Password placeholder={credentialPlaceholder} autoComplete="new-password" />
        </Form.Item>
        <Form.Item<AccountAddReq>
          label={
            <Space>
              账号代理
              <Tooltip title={"编辑已有账号时留空会保留当前账号代理；新账号留空时使用全局 transport.proxy_url。支持 http、https、socks5、socks5h。"}>
                <InfoCircleOutlined/>
              </Tooltip>
            </Space>
          }
          name="proxyUrl"
        >
          <Input.Password
            placeholder={proxyPlaceholder}
            autoComplete="new-password"
          />
        </Form.Item>
        <Form.Item<AccountAddReq>
          label={
            <Space>
              共享
              <Tooltip title={"共享记录会保留在本地；远端登录能力由 Provider 实现负责"} >
                <InfoCircleOutlined/>
              </Tooltip>
            </Space>
          }
          name="shared"
          labelAlign="left"
          valuePropName="checked"
          getValueFromEvent={(v) => {
            return v ? 1 : 0;
          }}
          required
        >
          <Switch />
        </Form.Item>
        {formValue.accountType === 'chatgpt' ? (
          <Form.Item<AccountAddReq>
            label={"OneAPI 渠道"}
            name={"oneApiChannelId"}
          >
            <Select
              placeholder={"对接 OneAPI/New API 的渠道，非必填"}
              allowClear={true}
              loading={isLoading}
              notFoundContent={isLoading ? <Spin size={"small"} /> : null}
            >
              {data?.map((item) => (
                <Select.Option key={item.id}>
                  <Space>
                    <Typography.Text>{item.name}</Typography.Text>
                    <div>
                      {
                        item.group.split(',').map((group) => {
                          const colors = ["volcano", "orange", "gold", "lime", "green", "cyan", "blue", "geekblue", "purple", "magenta", "red"];
                          return <Tag color={colors[group.charCodeAt(0) % colors.length]}
                            key={group}
                          >{group}</Tag>
                        })
                      }
                    </div>
                  </Space>
                </Select.Option>
              ))}
            </Select>
          </Form.Item>
        ): null}
        {formValue.accountType === 'chatgpt' ? (
          <>
            <Form.Item
              label="Session Token"
              name="sessionToken"
              tooltip="可填 __Secure-next-auth.session-token 的值。完整浏览器 Cookie 更适合分片 session cookie。"
            >
              <Input.Password placeholder={credentialPlaceholder} autoComplete="new-password" />
            </Form.Item>
            <Form.Item
              label="Browser Cookie"
              name="cookie"
              tooltip="可粘贴 chatgpt.com 请求的完整 Cookie header。只会加密保存，不会在账号列表接口中回显。"
            >
              <Input.TextArea
                placeholder={credentialPlaceholder}
                autoComplete="off"
                autoSize={{ minRows: 2, maxRows: 5 }}
              />
            </Form.Item>
            <Form.Item
              label="Refresh Token"
              name="refreshToken"
            >
              <Input.Password placeholder={credentialPlaceholder} autoComplete="new-password" />
            </Form.Item>
            <Form.Item
              label="Access Token"
              name="accessToken"
            >
              <Input.Password placeholder={credentialPlaceholder} autoComplete="new-password" />
            </Form.Item>
          </>
        ) : (
          <Form.Item
            label="Session Key"
            name="sessionKey"
          >
            <Input.Password placeholder={credentialPlaceholder} autoComplete="new-password" />
          </Form.Item>
        )}
      </Form>
    </Modal>
  );
}