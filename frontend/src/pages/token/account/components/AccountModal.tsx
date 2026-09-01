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
          <Password placeholder={"仅作本地备注/兼容字段"} />
        </Form.Item>
        <Form.Item<AccountAddReq>
          label={
            <Space>
              账号代理
              <Tooltip title={"留空时使用全局 transport.proxy_url；支持 http、https、socks5、socks5h。优先级：账号代理 > 全局代理 > 直连"}>
                <InfoCircleOutlined/>
              </Tooltip>
            </Space>
          }
          name="proxyUrl"
        >
          <Input.Password
            placeholder="例如 socks5h://user:pass@127.0.0.1:1080"
            autoComplete="new-password"
          />
        </Form.Item>
        <Form.Item<AccountAddReq>
          label={
            <Space>
              共享
              <Tooltip title={"共享记录会保留在本地；Provider 接入完成前无法生成远端登录会话"} >
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
              label="Refresh Token（本地保存，当前不会自动刷新）"
              name="refreshToken"
            >
              <Input.TextArea placeholder="Provider 接入完成后由 Provider 负责刷新流程" />
            </Form.Item>
            <Form.Item
              label="Access Token（手动填写）"
              name="accessToken"
            >
              <Input.TextArea placeholder="当前仅本地保存；不会发送到旧第三方网关" />
            </Form.Item>
          </>
        ) : (
          <Form.Item
            label="Session Key（兼容字段）"
            name="sessionKey"
          >
            <Input.TextArea placeholder="仅保留已有数据兼容，不再提供旧第三方获取入口" />
          </Form.Item>
        )}
      </Form>
    </Modal>
  );
}
