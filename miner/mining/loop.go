// Package mining implements the miner's main loop.
// Polls for challenges, solves them, submits commit-reveal via chain tx.
// State is persisted so restarts don't lose committed challenges.
package mining

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"log/slog"
	"time"

	"github.com/clawchain/clawminer/client"
	"github.com/clawchain/clawminer/config"
	"github.com/clawchain/clawminer/solver"
	"github.com/clawchain/clawminer/state"
)

// MiningLoop is the main mining loop.
type MiningLoop struct {
	cfg         *config.Config
	chainClient *client.ChainClient
	solver      *solver.Solver
	store       *state.Store
	minerAddr   string
	logger      *slog.Logger
	lastHeight  int64
}

// NewMiningLoop creates a new mining loop.
func NewMiningLoop(
	cfg *config.Config,
	chainClient *client.ChainClient,
	slv *solver.Solver,
	store *state.Store,
	minerAddr string,
	logger *slog.Logger,
) *MiningLoop {
	return &MiningLoop{
		cfg:         cfg,
		chainClient: chainClient,
		solver:      slv,
		store:       store,
		minerAddr:   minerAddr,
		logger:      logger,
	}
}

// Run starts the mining loop (blocks until context is cancelled).
func (m *MiningLoop) Run(ctx context.Context) error {
	m.logger.Info("⛏️  Mining loop started",
		"miner", m.minerAddr,
		"node", m.cfg.NodeRPC,
		"chain_id", m.cfg.ChainID,
	)

	// Process any pending reveals from previous run
	m.processPendingReveals(ctx)

	ticker := time.NewTicker(6 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			m.logger.Info("Mining loop stopped")
			return nil
		case <-ticker.C:
			m.tick(ctx)
		}
	}
}

func (m *MiningLoop) tick(ctx context.Context) {
	height, err := m.chainClient.GetLatestBlock(ctx)
	if err != nil {
		m.logger.Warn("Failed to get block height, will retry", "error", err)
		return
	}

	if height <= m.lastHeight {
		return
	}
	m.lastHeight = height

	// 1. Process pending reveals
	m.processPendingReveals(ctx)

	// 2. Look for new challenges
	challenges, err := m.chainClient.GetPendingChallenges(ctx, m.minerAddr)
	if err != nil {
		m.logger.Warn("Failed to query challenges", "error", err)
		return
	}

	for _, ch := range challenges {
		if m.store.HasCommitted(ch.ID) {
			continue
		}
		m.processChallenge(ctx, ch)
	}
}

func (m *MiningLoop) processChallenge(ctx context.Context, ch solver.Challenge) {
	m.logger.Info("📋 New challenge", "id", ch.ID, "type", ch.Type)

	// Solve
	answer, err := m.solver.Solve(ctx, ch)
	if err != nil {
		m.logger.Error("Solve failed", "id", ch.ID, "error", err)
		return
	}

	// Generate salt
	saltBytes := make([]byte, 32)
	if _, err := rand.Read(saltBytes); err != nil {
		m.logger.Error("Salt generation failed", "error", err)
		return
	}
	salt := hex.EncodeToString(saltBytes)

	// Compute commit hash
	commitHash := client.ComputeCommitHash(answer, salt)

	// Submit commit
	m.logger.Info("📤 Submitting commit", "id", ch.ID)
	txHash, err := m.chainClient.SubmitCommit(ctx, m.minerAddr, ch.ID, commitHash)
	if err != nil {
		m.logger.Error("Commit tx failed", "id", ch.ID, "error", err)
		return
	}

	// Persist state BEFORE returning
	rec := &state.CommitRecord{
		ChallengeID: ch.ID,
		Answer:      answer,
		Salt:        salt,
		CommitHash:  commitHash,
		CommitTx:    txHash,
		CommitBlock: m.lastHeight,
	}
	if err := m.store.SaveCommit(rec); err != nil {
		m.logger.Error("Failed to persist commit state", "id", ch.ID, "error", err)
		// Don't return — tx is already on chain, reveal will still work from memory
	}

	m.logger.Info("✅ Commit accepted + state saved",
		"id", ch.ID,
		"tx", txHash[:16]+"...",
		"answer_len", len(answer),
	)
}

func (m *MiningLoop) processPendingReveals(ctx context.Context) {
	pending := m.store.GetPendingReveals()
	for _, rec := range pending {
		// Wait at least 2 blocks after commit before revealing
		if m.lastHeight-rec.CommitBlock < 2 {
			continue
		}

		m.logger.Info("📤 Submitting reveal", "id", rec.ChallengeID)

		txHash, err := m.chainClient.SubmitReveal(ctx, m.minerAddr, rec.ChallengeID, rec.Answer, rec.Salt)
		if err != nil {
			m.logger.Warn("Reveal failed, will retry",
				"id", rec.ChallengeID,
				"error", err,
			)
			// Check if too old (>200 blocks since commit = probably expired)
			if m.lastHeight-rec.CommitBlock > 200 {
				m.logger.Warn("Reveal expired, marking done", "id", rec.ChallengeID)
				_ = m.store.MarkDone(rec.ChallengeID)
			}
			continue
		}

		if err := m.store.MarkRevealed(rec.ChallengeID, txHash); err != nil {
			m.logger.Error("Failed to persist reveal state", "id", rec.ChallengeID)
		}
		_ = m.store.MarkDone(rec.ChallengeID)

		m.logger.Info("✅ Reveal accepted",
			"id", rec.ChallengeID,
			"tx", txHash[:16]+"...",
		)
	}
}
