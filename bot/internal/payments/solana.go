package payments

import (
	"context"
	"fmt"
	"math"
	"net/http"
	"time"

	"github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/programs/system"
	"github.com/gagliardetto/solana-go/rpc"
)

type SolanaClient struct {
	rpc    *rpc.Client
	main   solana.PublicKey
	client *http.Client
}

func NewSolanaClient(rpcURL, mainWallet string) (*SolanaClient, error) {
	main, err := solana.PublicKeyFromBase58(mainWallet)
	if err != nil {
		return nil, fmt.Errorf("invalid main wallet: %w", err)
	}
	return &SolanaClient{
		rpc:    rpc.New(rpcURL),
		main:   main,
		client: &http.Client{Timeout: 15 * time.Second},
	}, nil
}

func GenerateDeposit() (address, privKey string, err error) {
	k, e := solana.NewRandomPrivateKey()
	if e != nil {
		return "", "", e
	}
	return k.PublicKey().String(), k.String(), nil
}

func (c *SolanaClient) Watch(ctx context.Context, depositAddr string, expectedLamports int64) (string, error) {
	addr, err := solana.PublicKeyFromBase58(depositAddr)
	if err != nil {
		return "", err
	}

	sigs, err := c.rpc.GetSignaturesForAddress(ctx, addr)
	if err != nil {
		return "", err
	}
	if len(sigs) == 0 {
		return "", nil
	}

	ver := uint64(0)
	for _, s := range sigs {
		if s.Err != nil {
			continue
		}
		res, err := c.rpc.GetTransaction(ctx, s.Signature, &rpc.GetTransactionOpts{
			Encoding:                       solana.EncodingBase64,
			MaxSupportedTransactionVersion: &ver,
		})
		if err != nil || res == nil || res.Meta == nil {
			continue
		}
		tx, err := res.Transaction.GetTransaction()
		if err != nil || tx == nil {
			continue
		}
		received, ok := receivedLamports(tx, addr, res.Meta)
		if !ok {
			continue
		}
		if received >= expectedLamports {
			return s.Signature.String(), nil
		}
	}
	return "", nil
}

func receivedLamports(tx *solana.Transaction, deposit solana.PublicKey, meta *rpc.TransactionMeta) (int64, bool) {
	idx := -1
	for i, a := range tx.Message.AccountKeys {
		if a.Equals(deposit) {
			idx = i
			break
		}
	}
	if idx < 0 || idx >= len(meta.PreBalances) || idx >= len(meta.PostBalances) {
		return 0, false
	}
	diff := int64(meta.PostBalances[idx] - meta.PreBalances[idx])
	return diff, diff > 0
}

func (c *SolanaClient) Sweep(ctx context.Context, depositPrivKey string) (string, error) {
	k, err := solana.PrivateKeyFromBase58(depositPrivKey)
	if err != nil {
		return "", err
	}
	from := k.PublicKey()

	bal, err := c.rpc.GetBalance(ctx, from, rpc.CommitmentFinalized)
	if err != nil {
		return "", err
	}
	if bal.Value <= 5000 {
		return "", nil
	}

	recent, err := c.rpc.GetLatestBlockhash(ctx, rpc.CommitmentFinalized)
	if err != nil {
		return "", err
	}

	ix := system.NewTransferInstruction(bal.Value-5000, from, c.main).Build()

	tx, err := solana.NewTransaction([]solana.Instruction{ix}, recent.Value.Blockhash,
		solana.TransactionPayer(from))
	if err != nil {
		return "", err
	}
	_, err = tx.Sign(func(pub solana.PublicKey) *solana.PrivateKey {
		if pub.Equals(from) {
			return &k
		}
		return nil
	})
	if err != nil {
		return "", err
	}

	sig, err := c.rpc.SendTransaction(ctx, tx)
	if err != nil {
		return "", err
	}
	return sig.String(), nil
}

func SOLToLamports(sol string) (int64, error) {
	var f float64
	if _, err := fmt.Sscanf(sol, "%f", &f); err != nil {
		return 0, err
	}
	return int64(math.Round(f * 1e9)), nil
}

const DefaultRPCURL = "https://api.mainnet-beta.solana.com"
