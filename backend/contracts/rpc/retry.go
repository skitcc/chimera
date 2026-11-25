// Package rpc holds call rules that are part of the gRPC contracts.
package rpc

import (
	catalogv1 "chimera/contracts/gen/catalog/v1"
	identityv1 "chimera/contracts/gen/identity/v1"
	socialv1 "chimera/contracts/gen/social/v1"
)

// RepeatUnsafe lists the methods a client must not repeat after an attempt timed
// out: the first attempt may have succeeded, and the repeat would then report a
// different result (not_found after a delete, a conflict after an add or a publish).
// A repeat after UNAVAILABLE is safe for every method, because the call did not run.
// Create* methods are not listed: idempotency_key makes their repeat return the first result.
var RepeatUnsafe = []string{
	identityv1.IdentityService_CreateSession_FullMethodName,
	identityv1.IdentityService_VerifySession_FullMethodName,
	identityv1.IdentityService_StartRecovery_FullMethodName,
	identityv1.IdentityService_StartPasswordChange_FullMethodName,
	identityv1.IdentityService_DeleteUser_FullMethodName,
	catalogv1.CatalogService_PatchTrack_FullMethodName,
	catalogv1.CatalogService_DeleteTrack_FullMethodName,
	socialv1.SocialService_DeletePlaylist_FullMethodName,
	socialv1.SocialService_AddPlaylistTrack_FullMethodName,
	socialv1.SocialService_RemovePlaylistTrack_FullMethodName,
}
