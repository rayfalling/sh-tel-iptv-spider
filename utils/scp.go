package utils

import (
	"errors"
	"fmt"
	"os/exec"
)

// ScpCopy 使用 scp 命令拷贝文件到远程服务器。
//
// 认证方式：
//   - password 非空：通过 sshpass 非交互认证（需系统已安装 sshpass），未安装则直接返回可读错误
//   - password 为空：沿用调用方已有的 SSH 密钥 / agent
//
// 原实现声明了 password 参数却从未使用，导致 scp 在无 tty 环境下交互式索要密码而卡住。
func ScpCopy(localPath, remotePath, host, user, password string) error {
	if host == "" || user == "" {
		return errors.New("scp 参数不完整: 需要 host 与 user")
	}

	var cmd *exec.Cmd
	if password != "" {
		if _, err := exec.LookPath("sshpass"); err == nil {
			cmd = exec.Command("sshpass", "-p", password,
				"scp",
				"-o", "StrictHostKeyChecking=no",
				"-o", "UserKnownHostsFile=/dev/null",
				localPath,
				fmt.Sprintf("%s@%s:%s", user, host, remotePath),
			)
		} else {
			return errors.New("已配置 SCP 密码但系统未安装 sshpass，无法非交互认证；请安装 sshpass 或改用 SSH 密钥")
		}
	} else {
		cmd = exec.Command("scp",
			"-o", "StrictHostKeyChecking=no",
			"-o", "UserKnownHostsFile=/dev/null",
			localPath,
			fmt.Sprintf("%s@%s:%s", user, host, remotePath),
		)
	}

	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("scp failed: %v, output: %s", err, string(output))
	}
	return nil
}
