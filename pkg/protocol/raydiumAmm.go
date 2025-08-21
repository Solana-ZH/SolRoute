package protocol

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"fmt"

	"github.com/Solana-ZH/solroute/pkg"
	"github.com/Solana-ZH/solroute/pkg/pool/raydium"
	"github.com/Solana-ZH/solroute/pkg/sol"
	"github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/rpc"
)

type RaydiumAMMProtocol struct {
	SolClient *sol.Client
}

func NewRaydiumAmm(solClient *sol.Client) *RaydiumAMMProtocol {
	return &RaydiumAMMProtocol{
		SolClient: solClient,
	}
}

func (p *RaydiumAMMProtocol) FetchPoolsByPair(ctx context.Context, baseMint, quoteMint string) ([]pkg.Pool, error) {
	accounts := make([]*rpc.KeyedAccount, 0)
	programAccounts, err := p.getAMMPoolAccountsByTokenPair(ctx, baseMint, quoteMint)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch pools with base token %s: %w", baseMint, err)
	}
	accounts = append(accounts, programAccounts...)
	programAccounts, err = p.getAMMPoolAccountsByTokenPair(ctx, quoteMint, baseMint)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch pools with base token %s: %w", quoteMint, err)
	}
	accounts = append(accounts, programAccounts...)

	res := make([]pkg.Pool, 0)
	for _, v := range accounts {
		layout := &raydium.AMMPool{}
		if err := layout.Decode(v.Account.Data.GetBinary()); err != nil {
			continue
		}
		layout.PoolId = v.Pubkey
		if err := p.processAMMPool(ctx, layout); err != nil {
			return nil, fmt.Errorf("failed to process AMM pool %s: %w", v.Pubkey.String(), err)
		}
		res = append(res, layout)
	}
	return res, nil
}

func (p *RaydiumAMMProtocol) getAMMPoolAccountsByTokenPair(ctx context.Context, baseMint string, quoteMint string) (rpc.GetProgramAccountsResult, error) {
	var layout raydium.AMMPool
	baseMintPubkey, err := solana.PublicKeyFromBase58(baseMint)
	if err != nil {
		return nil, fmt.Errorf("invalid base mint address: %w", err)
	}
	quoteMintPubkey, err := solana.PublicKeyFromBase58(quoteMint)
	if err != nil {
		return nil, fmt.Errorf("invalid quote mint address: %w", err)
	}

	return p.SolClient.RpcClient.GetProgramAccountsWithOpts(ctx, raydium.RAYDIUM_AMM_PROGRAM_ID, &rpc.GetProgramAccountsOpts{
		Filters: []rpc.RPCFilter{
			{
				DataSize: layout.Span(),
			},
			{
				Memcmp: &rpc.RPCFilterMemcmp{
					Offset: layout.Offset("BaseMint"),
					Bytes:  baseMintPubkey.Bytes(),
				},
			},
			{
				Memcmp: &rpc.RPCFilterMemcmp{
					Offset: layout.Offset("QuoteMint"),
					Bytes:  quoteMintPubkey.Bytes(),
				},
			},
		},
	})
}

// FetchPoolByID fetches a specific pool by its ID
func (r *RaydiumAMMProtocol) FetchPoolByID(ctx context.Context, poolID string) (pkg.Pool, error) {
	poolPubkey, err := solana.PublicKeyFromBase58(poolID)
	if err != nil {
		return nil, fmt.Errorf("invalid pool ID: %w", err)
	}

	account, err := r.SolClient.RpcClient.GetAccountInfo(ctx, poolPubkey)
	if err != nil {
		return nil, fmt.Errorf("failed to get pool account %s: %w", poolID, err)
	}

	layout := &raydium.AMMPool{}
	if err := layout.Decode(account.Value.Data.GetBinary()); err != nil {
		return nil, fmt.Errorf("failed to decode pool data for %s: %w", poolID, err)
	}
	layout.PoolId = poolPubkey
	if err := r.processAMMPool(ctx, layout); err != nil {
		return nil, fmt.Errorf("failed to process AMM pool %s: %w", poolID, err)
	}
	return layout, nil
}

func getAssociatedAuthority(programID solana.PublicKey, marketID solana.PublicKey) (solana.PublicKey, uint8, error) {
	seeds := [][]byte{marketID.Bytes()}
	var nonce uint8 = 0

	for nonce < 100 {
		seedsWithNonce := append(seeds, int8ToBuf(nonce))
		seedsWithNonce = append(seedsWithNonce, make([]byte, 7))

		publicKey, err := solana.CreateProgramAddress(seedsWithNonce, programID)
		if err != nil {
			nonce++
			continue
		}

		return publicKey, nonce, nil
	}

	return solana.PublicKey{}, 0, errors.New("unable to find a viable program address nonce")
}

func int8ToBuf(value uint8) []byte {
	buf := new(bytes.Buffer)
	binary.Write(buf, binary.LittleEndian, value)
	return buf.Bytes()
}

func (p *RaydiumAMMProtocol) processAMMPool(ctx context.Context, layout *raydium.AMMPool) error {
	marketAccount, err := p.SolClient.RpcClient.GetAccountInfo(ctx, layout.MarketId)
	if err != nil {
		return fmt.Errorf("failed to get market account: %w", err)
	}

	var marketLayout raydium.MarketStateLayoutV3
	if err := marketLayout.Decode(marketAccount.Value.Data.GetBinary()); err != nil {
		return fmt.Errorf("failed to decode market layout: %w", err)
	}

	authority, _, err := solana.FindProgramAddress([][]byte{{97, 109, 109, 32, 97, 117, 116, 104, 111, 114, 105, 116, 121}}, raydium.RAYDIUM_AMM_PROGRAM_ID)
	if err != nil {
		return fmt.Errorf("failed to find program address: %w", err)
	}

	marketAuthority, _, err := getAssociatedAuthority(marketAccount.Value.Owner, marketLayout.OwnAddress)
	if err != nil {
		return fmt.Errorf("failed to get associated authority: %w", err)
	}

	layout.Authority = authority
	layout.MarketAuthority = marketAuthority
	return nil
}
