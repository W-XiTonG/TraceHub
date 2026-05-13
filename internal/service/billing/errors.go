package billing

import "errors"

// ErrInsufficientBalance 原子扣费时余额不足（未修改余额）。
var ErrInsufficientBalance = errors.New("insufficient balance")
