// chrome.d.ts —— 最小化 chrome.* API 类型声明
// 项目约定 devDependency 仅 typescript，不引入 @types/chrome，
// 这里只声明本扩展实际用到的 API（storage / cookies / alarms / runtime）。
// TODO: 后续若用到更多 chrome API，可考虑改用 @types/chrome。

declare namespace chrome {
  namespace storage {
    interface StorageChange {
      oldValue?: unknown;
      newValue?: unknown;
    }
    interface StorageArea {
      // 传默认值对象时，返回合并了默认值的结果
      get(keys: string | string[] | Record<string, unknown> | null): Promise<Record<string, unknown>>;
      set(items: Record<string, unknown>): Promise<void>;
      remove(keys: string | string[]): Promise<void>;
    }
    const sync: StorageArea;
    const local: StorageArea;
    const onChanged: {
      addListener(callback: (changes: Record<string, StorageChange>, areaName: string) => void): void;
    };
  }

  namespace cookies {
    interface Cookie {
      domain: string;
      name: string;
      value: string;
      path?: string;
      secure?: boolean;
      httpOnly?: boolean;
      session?: boolean;
      expirationDate?: number;
      sameSite?: string;
    }
    interface CookieChangeInfo {
      cookie: Cookie;
      cause: string;
      removed: boolean;
    }
    function getAll(details: { domain?: string; name?: string; url?: string }): Promise<Cookie[]>;
    const onChanged: {
      addListener(callback: (changeInfo: CookieChangeInfo) => void): void;
    };
  }

  namespace alarms {
    interface Alarm {
      name: string;
      scheduledTime: number;
      periodInMinutes?: number;
    }
    function create(
      name: string,
      alarmInfo?: { when?: number; delayInMinutes?: number; periodInMinutes?: number }
    ): Promise<void>;
    function clear(name: string): Promise<boolean>;
    const onAlarm: {
      addListener(callback: (alarm: Alarm) => void): void;
    };
  }

  namespace runtime {
    interface MessageSender {
      id?: string;
      url?: string;
    }
    function sendMessage(message: unknown): Promise<unknown>;
    function openOptionsPage(): Promise<void>;
    const onInstalled: {
      addListener(callback: (details: { reason: string; previousVersion?: string }) => void): void;
    };
    const onMessage: {
      addListener(
        callback: (
          message: unknown,
          sender: MessageSender,
          sendResponse: (response?: unknown) => void
        ) => boolean | void
      ): void;
    };
    const lastError: { message?: string } | undefined;
  }

  namespace tabs {
    interface Tab {
      id?: number;
      url?: string;
      title?: string;
      active?: boolean;
      windowId?: number;
    }
    function query(queryInfo: { url?: string | string[] }): Promise<Tab[]>;
    function create(createProperties: { url?: string; active?: boolean }): Promise<Tab>;
    function update(tabId: number, updateProperties: { active?: boolean; url?: string }): Promise<Tab>;
    function sendMessage(tabId: number, message: unknown): Promise<unknown>;
  }
}
