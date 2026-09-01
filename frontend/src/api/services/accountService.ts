import apiClient from '../apiClient';

import {Account, ProductType} from '#/entity';

export enum AccountApi {
  list = '/account/list',
  add = '/account/add',
  update = '/account/update',
  delete = '/account/delete',
  refresh = '/account/refresh',
  search = '/account/search',
  shareAccounts = '/share_accounts',
  loginFree = '/login_free_account',
  getOneApiChannel = '/account/oneapi/channels',
}

const getAccountList = () =>
  apiClient.get<Account[]>({ url: AccountApi.list }).then((res) => {
    res.forEach((item) => {
      if (item.shareList) {
        item.shareList = JSON.parse(item.shareList);
      }
    });
    return res;
  });

const searchAccountList = (email: string, accountType: ProductType ) =>
  apiClient.post<Account[]>({ url: AccountApi.search, data: { email, accountType } }).then((res) => {
    res.forEach((item) => {
      if (item.shareList) {
        item.shareList = JSON.parse(item.shareList);
      }
    });
    return res;
  });

export interface AccountAddReq {
  id?: number;
  email: string;
  password?: string;
  shared?: number;
  accountType: ProductType;
  sessionToken?: string;
  sessionKey?: string;
  refreshToken?: string;
  accessToken?: string;
  oneApiChannelId?: string | number;
  proxyUrl?: string;
  proxyDisplay?: string;
  hasCredential?: boolean;
}

interface ShareAccountListResp {
  accounts: Account[];
  random: boolean;
  custom: boolean;
}

interface LoginFreeAccountResp {
  id: number,
  UniqueName?: string,
  SelectType?: string,
}

interface OneApiChannel {
  id: number;
  name: string;
  group: string;
  status: number;
}

const normalizeWriteRequest = (data: AccountAddReq) => ({
  ...data,
  oneApiChannelId: data.oneApiChannelId == null ? '' : String(data.oneApiChannelId),
  proxyDisplay: undefined,
  hasCredential: undefined,
});

const addAccount = (data: AccountAddReq) => apiClient.post({ url: AccountApi.add, data: normalizeWriteRequest(data) });
const updateAccount = (data: AccountAddReq) => apiClient.post({ url: AccountApi.update, data: normalizeWriteRequest(data) });
const deleteAccount = (id: number) => apiClient.post({ url: AccountApi.delete, data: { id } });
const refreshAccount = (id: number) => apiClient.post({ url: AccountApi.refresh, data: { id } });
const getShareAccountList = () => apiClient.post<ShareAccountListResp>({ url: AccountApi.shareAccounts });
const getOneApiChannelList = () => apiClient.post<OneApiChannel[]>({ url: AccountApi.getOneApiChannel });
const loginFreeAccount = (data: LoginFreeAccountResp) => apiClient.post({ url: AccountApi.loginFree, data });

export default {
  getOneApiChannelList,
  getAccountList,
  searchAccountList,
  addAccount,
  updateAccount,
  deleteAccount,
  refreshAccount,
  getShareAccountList,
  loginFreeAccount
};
