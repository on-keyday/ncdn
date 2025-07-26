; ModuleID = 'init_crypto.c'
source_filename = "init_crypto.c"
target datalayout = "e-m:e-p270:32:32-p271:32:32-p272:64:64-i64:64-f80:128-n8:16:32:64-S128"
target triple = "x86_64-pc-linux-gnu"

%struct.bpf_crypto_ctx = type { %struct.bpf_crypto_type*, i8*, i32, %struct.callback_head, %struct.refcount_struct }
%struct.bpf_crypto_type = type { i8* (i8*)*, void (i8*)*, i32 (i8*)*, i32 (i8*, i8*, i32)*, i32 (i8*, i32)*, i32 (i8*, i8*, i8*, i32, i8*)*, i32 (i8*, i8*, i8*, i32, i8*)*, i32 (i8*)*, i32 (i8*)*, i32 (i8*)*, i8*, [14 x i8] }
%struct.callback_head = type { %struct.callback_head*, void (%struct.callback_head*)* }
%struct.refcount_struct = type { %struct.atomic_t }
%struct.atomic_t = type { i32 }
%struct.bpf_crypto_params = type { [14 x i8], [2 x i8], [128 x i8], [256 x i8], i32, i32 }
%struct.array_map = type { [2 x i32]*, i32*, %struct.__crypto_ctx_value*, [1 x i32]* }
%struct.__crypto_ctx_value = type { %struct.bpf_crypto_ctx* }

@value = dso_local global %struct.bpf_crypto_ctx zeroinitializer, align 8, !dbg !0
@__const.crypto_init.params = private unnamed_addr constant %struct.bpf_crypto_params { [14 x i8] c"skcipher\00\00\00\00\00\00", [2 x i8] zeroinitializer, [128 x i8] c"ecb\00\00\00\00\00\00\00\00\00\00\00\00\00\00\00\00\00\00\00\00\00\00\00\00\00\00\00\00\00\00\00\00\00\00\00\00\00\00\00\00\00\00\00\00\00\00\00\00\00\00\00\00\00\00\00\00\00\00\00\00\00\00\00\00\00\00\00\00\00\00\00\00\00\00\00\00\00\00\00\00\00\00\00\00\00\00\00\00\00\00\00\00\00\00\00\00\00\00\00\00\00\00\00\00\00\00\00\00\00\00\00\00\00\00\00\00\00\00\00\00\00\00\00\00\00", [256 x i8] zeroinitializer, i32 16, i32 0 }, align 4
@status = dso_local local_unnamed_addr global i32 0, align 4, !dbg !117
@_license = dso_local global [13 x i8] c"Dual BSD/GPL\00", section "license", align 1, !dbg !5
@__crypto_ctx_map = dso_local global %struct.array_map zeroinitializer, section ".maps", align 8, !dbg !11
@dummy = dso_local local_unnamed_addr global %struct.__crypto_ctx_value zeroinitializer, align 8, !dbg !119
@llvm.compiler.used = appending global [3 x i8*] [i8* bitcast (%struct.array_map* @__crypto_ctx_map to i8*), i8* getelementptr inbounds ([13 x i8], [13 x i8]* @_license, i32 0, i32 0), i8* bitcast (i32 (i8*)* @crypto_init to i8*)], section "llvm.metadata"

; Function Attrs: nounwind uwtable
define dso_local i32 @crypto_init(i8* nocapture readnone %0) #0 section "syscall" !dbg !153 {
  %2 = alloca %struct.__crypto_ctx_value, align 8
  %3 = alloca i32, align 4
  %4 = alloca %struct.bpf_crypto_ctx, align 8
  %5 = alloca %struct.bpf_crypto_params, align 4
  %6 = alloca i32, align 4
  call void @llvm.dbg.value(metadata i8* undef, metadata !157, metadata !DIExpression()), !dbg !177
  %7 = bitcast %struct.bpf_crypto_ctx* %4 to i8*, !dbg !178
  call void @llvm.memcpy.p0i8.p0i8.i64(i8* nonnull align 8 %7, i8* align 8 bitcast (%struct.bpf_crypto_ctx* @value to i8*), i64 48, i1 true), !dbg !178, !tbaa.struct !179
  %8 = getelementptr inbounds %struct.bpf_crypto_params, %struct.bpf_crypto_params* %5, i64 0, i32 0, i64 0, !dbg !186
  call void @llvm.lifetime.start.p0i8(i64 408, i8* nonnull %8) #6, !dbg !186
  call void @llvm.dbg.declare(metadata %struct.bpf_crypto_params* %5, metadata !160, metadata !DIExpression()), !dbg !187
  call void @llvm.memcpy.p0i8.p0i8.i64(i8* noundef nonnull align 4 dereferenceable(408) %8, i8* noundef nonnull align 4 dereferenceable(408) getelementptr inbounds (%struct.bpf_crypto_params, %struct.bpf_crypto_params* @__const.crypto_init.params, i64 0, i32 0, i64 0), i64 408, i1 false), !dbg !187
  %9 = bitcast i32* %6 to i8*, !dbg !188
  call void @llvm.lifetime.start.p0i8(i64 4, i8* nonnull %9) #6, !dbg !188
  call void @llvm.dbg.value(metadata i32 0, metadata !176, metadata !DIExpression()), !dbg !177
  store i32 0, i32* %6, align 4, !dbg !189, !tbaa !184
  store i32 0, i32* @status, align 4, !dbg !190, !tbaa !184
  call void @llvm.dbg.value(metadata i32* %6, metadata !176, metadata !DIExpression(DW_OP_deref)), !dbg !177
  %10 = call %struct.bpf_crypto_ctx* @bpf_crypto_ctx_create(%struct.bpf_crypto_params* noundef nonnull %5, i32 noundef 408, i32* noundef nonnull %6) #7, !dbg !191
  call void @llvm.dbg.value(metadata %struct.bpf_crypto_ctx* %10, metadata !158, metadata !DIExpression()), !dbg !177
  %11 = icmp eq %struct.bpf_crypto_ctx* %10, null, !dbg !192
  br i1 %11, label %12, label %14, !dbg !194

12:                                               ; preds = %1
  %13 = load i32, i32* %6, align 4, !dbg !195, !tbaa !184
  call void @llvm.dbg.value(metadata i32 %13, metadata !176, metadata !DIExpression()), !dbg !177
  store i32 %13, i32* @status, align 4, !dbg !197, !tbaa !184
  br label %36, !dbg !198

14:                                               ; preds = %1
  call void @llvm.dbg.value(metadata %struct.bpf_crypto_ctx* %10, metadata !199, metadata !DIExpression()) #6, !dbg !210
  %15 = bitcast %struct.__crypto_ctx_value* %2 to i8*, !dbg !212
  call void @llvm.lifetime.start.p0i8(i64 8, i8* nonnull %15) #6, !dbg !212
  call void @llvm.dbg.declare(metadata %struct.__crypto_ctx_value* %2, metadata !205, metadata !DIExpression()) #6, !dbg !213
  %16 = bitcast i32* %3 to i8*, !dbg !214
  call void @llvm.lifetime.start.p0i8(i64 4, i8* nonnull %16) #6, !dbg !214
  call void @llvm.dbg.value(metadata i32 0, metadata !208, metadata !DIExpression()) #6, !dbg !210
  store i32 0, i32* %3, align 4, !dbg !215, !tbaa !184
  %17 = getelementptr inbounds %struct.__crypto_ctx_value, %struct.__crypto_ctx_value* %2, i64 0, i32 0, !dbg !216
  store %struct.bpf_crypto_ctx* null, %struct.bpf_crypto_ctx** %17, align 8, !dbg !217, !tbaa !218
  call void @llvm.dbg.value(metadata i32* %3, metadata !208, metadata !DIExpression(DW_OP_deref)) #6, !dbg !210
  %18 = call i64 inttoptr (i64 2 to i64 (i8*, i8*, i8*, i64)*)(i8* noundef bitcast (%struct.array_map* @__crypto_ctx_map to i8*), i8* noundef nonnull %16, i8* noundef nonnull %15, i64 noundef 0) #7, !dbg !220
  %19 = trunc i64 %18 to i32, !dbg !220
  call void @llvm.dbg.value(metadata i32 %19, metadata !209, metadata !DIExpression()) #6, !dbg !210
  %20 = icmp eq i32 %19, 0, !dbg !221
  br i1 %20, label %21, label %32, !dbg !223

21:                                               ; preds = %14
  call void @llvm.dbg.value(metadata i32* %3, metadata !208, metadata !DIExpression(DW_OP_deref)) #6, !dbg !210
  %22 = call i8* inttoptr (i64 1 to i8* (i8*, i8*)*)(i8* noundef bitcast (%struct.array_map* @__crypto_ctx_map to i8*), i8* noundef nonnull %16) #7, !dbg !224
  call void @llvm.dbg.value(metadata i8* %22, metadata !206, metadata !DIExpression()) #6, !dbg !210
  %23 = icmp eq i8* %22, null, !dbg !225
  br i1 %23, label %24, label %25, !dbg !227

24:                                               ; preds = %21
  call void @bpf_crypto_ctx_release(%struct.bpf_crypto_ctx* noundef nonnull %10) #7, !dbg !228
  call void @llvm.lifetime.end.p0i8(i64 4, i8* nonnull %16) #6, !dbg !230
  call void @llvm.lifetime.end.p0i8(i64 8, i8* nonnull %15) #6, !dbg !230
  call void @llvm.dbg.value(metadata i32 %19, metadata !176, metadata !DIExpression()), !dbg !177
  br label %34, !dbg !231

25:                                               ; preds = %21
  call void @llvm.dbg.value(metadata i8* %22, metadata !206, metadata !DIExpression()) #6, !dbg !210
  %26 = bitcast %struct.bpf_crypto_ctx* %10 to i8*, !dbg !233
  %27 = call i8* inttoptr (i64 194 to i8* (i8*, i8*)*)(i8* noundef nonnull %22, i8* noundef nonnull %26) #7, !dbg !234
  call void @llvm.dbg.value(metadata i8* %27, metadata !207, metadata !DIExpression()) #6, !dbg !210
  %28 = icmp eq i8* %27, null, !dbg !235
  br i1 %28, label %31, label %29, !dbg !237

29:                                               ; preds = %25
  %30 = bitcast i8* %27 to %struct.bpf_crypto_ctx*, !dbg !234
  call void @llvm.dbg.value(metadata %struct.bpf_crypto_ctx* %30, metadata !207, metadata !DIExpression()) #6, !dbg !210
  call void @bpf_crypto_ctx_release(%struct.bpf_crypto_ctx* noundef nonnull %30) #7, !dbg !238
  br label %31, !dbg !240

31:                                               ; preds = %29, %25
  call void @llvm.lifetime.end.p0i8(i64 4, i8* nonnull %16) #6, !dbg !230
  call void @llvm.lifetime.end.p0i8(i64 8, i8* nonnull %15) #6, !dbg !230
  call void @llvm.dbg.value(metadata i32 %19, metadata !176, metadata !DIExpression()), !dbg !177
  br label %36, !dbg !231

32:                                               ; preds = %14
  call void @bpf_crypto_ctx_release(%struct.bpf_crypto_ctx* noundef nonnull %10) #7, !dbg !241
  call void @llvm.lifetime.end.p0i8(i64 4, i8* nonnull %16) #6, !dbg !230
  call void @llvm.lifetime.end.p0i8(i64 8, i8* nonnull %15) #6, !dbg !230
  call void @llvm.dbg.value(metadata i32 %19, metadata !176, metadata !DIExpression()), !dbg !177
  %33 = icmp eq i32 %19, -17, !dbg !231
  br i1 %33, label %36, label %34, !dbg !231

34:                                               ; preds = %32, %24
  %35 = phi i32 [ -2, %24 ], [ %19, %32 ]
  store i32 %35, i32* @status, align 4, !dbg !243, !tbaa !184
  br label %36, !dbg !244

36:                                               ; preds = %32, %31, %34, %12
  call void @llvm.lifetime.end.p0i8(i64 4, i8* nonnull %9) #6, !dbg !245
  call void @llvm.lifetime.end.p0i8(i64 408, i8* nonnull %8) #6, !dbg !245
  ret i32 0, !dbg !245
}

; Function Attrs: mustprogress nofree nosync nounwind readnone speculatable willreturn
declare void @llvm.dbg.declare(metadata, metadata, metadata) #1

; Function Attrs: argmemonly mustprogress nofree nounwind willreturn
declare void @llvm.memcpy.p0i8.p0i8.i64(i8* noalias nocapture writeonly, i8* noalias nocapture readonly, i64, i1 immarg) #2

; Function Attrs: argmemonly mustprogress nofree nosync nounwind willreturn
declare void @llvm.lifetime.start.p0i8(i64 immarg, i8* nocapture) #3

declare !dbg !246 %struct.bpf_crypto_ctx* @bpf_crypto_ctx_create(%struct.bpf_crypto_params* noundef, i32 noundef, i32* noundef) local_unnamed_addr #4 section ".ksyms"

; Function Attrs: argmemonly mustprogress nofree nosync nounwind willreturn
declare void @llvm.lifetime.end.p0i8(i64 immarg, i8* nocapture) #3

declare !dbg !252 void @bpf_crypto_ctx_release(%struct.bpf_crypto_ctx* noundef) local_unnamed_addr #4 section ".ksyms"

; Function Attrs: nofree nosync nounwind readnone speculatable willreturn
declare void @llvm.dbg.value(metadata, metadata, metadata) #5

attributes #0 = { nounwind uwtable "frame-pointer"="none" "min-legal-vector-width"="0" "no-builtin-bcmp" "no-trapping-math"="true" "stack-protector-buffer-size"="8" "target-cpu"="x86-64" "target-features"="+cx8,+fxsr,+mmx,+sse,+sse2,+x87" "tune-cpu"="generic" }
attributes #1 = { mustprogress nofree nosync nounwind readnone speculatable willreturn }
attributes #2 = { argmemonly mustprogress nofree nounwind willreturn }
attributes #3 = { argmemonly mustprogress nofree nosync nounwind willreturn }
attributes #4 = { "frame-pointer"="none" "no-builtin-bcmp" "no-trapping-math"="true" "stack-protector-buffer-size"="8" "target-cpu"="x86-64" "target-features"="+cx8,+fxsr,+mmx,+sse,+sse2,+x87" "tune-cpu"="generic" }
attributes #5 = { nofree nosync nounwind readnone speculatable willreturn }
attributes #6 = { nounwind }
attributes #7 = { nounwind "no-builtin-bcmp" }

!llvm.dbg.cu = !{!2}
!llvm.module.flags = !{!146, !147, !148, !149, !150, !151}
!llvm.ident = !{!152}

!0 = !DIGlobalVariableExpression(var: !1, expr: !DIExpression())
!1 = distinct !DIGlobalVariable(name: "value", scope: !2, file: !3, line: 32, type: !144, isLocal: false, isDefinition: true)
!2 = distinct !DICompileUnit(language: DW_LANG_C99, file: !3, producer: "Debian clang version 14.0.6", isOptimized: true, runtimeVersion: 0, emissionKind: FullDebug, globals: !4, splitDebugInlining: false, nameTableKind: None)
!3 = !DIFile(filename: "init_crypto.c", directory: "/workspaces/ncdn/l4lb/c", checksumkind: CSK_MD5, checksum: "f1e6e7341545ca9a0231a6f9ecaea52a")
!4 = !{!5, !11, !117, !0, !119, !122, !134, !139}
!5 = !DIGlobalVariableExpression(var: !6, expr: !DIExpression())
!6 = distinct !DIGlobalVariable(name: "_license", scope: !2, file: !3, line: 64, type: !7, isLocal: false, isDefinition: true)
!7 = !DICompositeType(tag: DW_TAG_array_type, baseType: !8, size: 104, elements: !9)
!8 = !DIBasicType(name: "char", size: 8, encoding: DW_ATE_signed_char)
!9 = !{!10}
!10 = !DISubrange(count: 13)
!11 = !DIGlobalVariableExpression(var: !12, expr: !DIExpression())
!12 = distinct !DIGlobalVariable(name: "__crypto_ctx_map", scope: !2, file: !13, line: 70, type: !14, isLocal: false, isDefinition: true)
!13 = !DIFile(filename: "./lb.h", directory: "/workspaces/ncdn/l4lb/c", checksumkind: CSK_MD5, checksum: "8cfdeab73634fe554b68a7b14bb420de")
!14 = distinct !DICompositeType(tag: DW_TAG_structure_type, name: "array_map", file: !13, line: 65, size: 256, elements: !15)
!15 = !{!16, !22, !24, !112}
!16 = !DIDerivedType(tag: DW_TAG_member, name: "type", scope: !14, file: !13, line: 66, baseType: !17, size: 64)
!17 = !DIDerivedType(tag: DW_TAG_pointer_type, baseType: !18, size: 64)
!18 = !DICompositeType(tag: DW_TAG_array_type, baseType: !19, size: 64, elements: !20)
!19 = !DIBasicType(name: "int", size: 32, encoding: DW_ATE_signed)
!20 = !{!21}
!21 = !DISubrange(count: 2)
!22 = !DIDerivedType(tag: DW_TAG_member, name: "key", scope: !14, file: !13, line: 67, baseType: !23, size: 64, offset: 64)
!23 = !DIDerivedType(tag: DW_TAG_pointer_type, baseType: !19, size: 64)
!24 = !DIDerivedType(tag: DW_TAG_member, name: "value", scope: !14, file: !13, line: 68, baseType: !25, size: 64, offset: 128)
!25 = !DIDerivedType(tag: DW_TAG_pointer_type, baseType: !26, size: 64)
!26 = distinct !DICompositeType(tag: DW_TAG_structure_type, name: "__crypto_ctx_value", file: !13, line: 60, size: 64, elements: !27)
!27 = !{!28}
!28 = !DIDerivedType(tag: DW_TAG_member, name: "ctx", scope: !26, file: !13, line: 61, baseType: !29, size: 64)
!29 = !DIDerivedType(tag: DW_TAG_pointer_type, baseType: !30, size: 64, annotations: !110)
!30 = distinct !DICompositeType(tag: DW_TAG_structure_type, name: "bpf_crypto_ctx", file: !13, line: 42, size: 384, elements: !31)
!31 = !{!32, !90, !91, !92, !101}
!32 = !DIDerivedType(tag: DW_TAG_member, name: "type", scope: !30, file: !13, line: 43, baseType: !33, size: 64)
!33 = !DIDerivedType(tag: DW_TAG_pointer_type, baseType: !34, size: 64)
!34 = !DIDerivedType(tag: DW_TAG_const_type, baseType: !35)
!35 = distinct !DICompositeType(tag: DW_TAG_structure_type, name: "bpf_crypto_type", file: !13, line: 28, size: 832, elements: !36)
!36 = !{!37, !44, !48, !52, !64, !68, !73, !74, !78, !79, !85, !86}
!37 = !DIDerivedType(tag: DW_TAG_member, name: "alloc_tfm", scope: !35, file: !13, line: 29, baseType: !38, size: 64)
!38 = !DIDerivedType(tag: DW_TAG_pointer_type, baseType: !39, size: 64)
!39 = !DISubroutineType(types: !40)
!40 = !{!41, !42}
!41 = !DIDerivedType(tag: DW_TAG_pointer_type, baseType: null, size: 64)
!42 = !DIDerivedType(tag: DW_TAG_pointer_type, baseType: !43, size: 64)
!43 = !DIDerivedType(tag: DW_TAG_const_type, baseType: !8)
!44 = !DIDerivedType(tag: DW_TAG_member, name: "free_tfm", scope: !35, file: !13, line: 30, baseType: !45, size: 64, offset: 64)
!45 = !DIDerivedType(tag: DW_TAG_pointer_type, baseType: !46, size: 64)
!46 = !DISubroutineType(types: !47)
!47 = !{null, !41}
!48 = !DIDerivedType(tag: DW_TAG_member, name: "has_algo", scope: !35, file: !13, line: 31, baseType: !49, size: 64, offset: 128)
!49 = !DIDerivedType(tag: DW_TAG_pointer_type, baseType: !50, size: 64)
!50 = !DISubroutineType(types: !51)
!51 = !{!19, !42}
!52 = !DIDerivedType(tag: DW_TAG_member, name: "setkey", scope: !35, file: !13, line: 32, baseType: !53, size: 64, offset: 192)
!53 = !DIDerivedType(tag: DW_TAG_pointer_type, baseType: !54, size: 64)
!54 = !DISubroutineType(types: !55)
!55 = !{!19, !41, !56, !63}
!56 = !DIDerivedType(tag: DW_TAG_pointer_type, baseType: !57, size: 64)
!57 = !DIDerivedType(tag: DW_TAG_const_type, baseType: !58)
!58 = !DIDerivedType(tag: DW_TAG_typedef, name: "uint8_t", file: !59, line: 24, baseType: !60)
!59 = !DIFile(filename: "/usr/include/x86_64-linux-gnu/bits/stdint-uintn.h", directory: "", checksumkind: CSK_MD5, checksum: "2bf2ae53c58c01b1a1b9383b5195125c")
!60 = !DIDerivedType(tag: DW_TAG_typedef, name: "__uint8_t", file: !61, line: 38, baseType: !62)
!61 = !DIFile(filename: "/usr/include/x86_64-linux-gnu/bits/types.h", directory: "", checksumkind: CSK_MD5, checksum: "d108b5f93a74c50510d7d9bc0ab36df9")
!62 = !DIBasicType(name: "unsigned char", size: 8, encoding: DW_ATE_unsigned_char)
!63 = !DIBasicType(name: "unsigned int", size: 32, encoding: DW_ATE_unsigned)
!64 = !DIDerivedType(tag: DW_TAG_member, name: "setauthsize", scope: !35, file: !13, line: 33, baseType: !65, size: 64, offset: 256)
!65 = !DIDerivedType(tag: DW_TAG_pointer_type, baseType: !66, size: 64)
!66 = !DISubroutineType(types: !67)
!67 = !{!19, !41, !63}
!68 = !DIDerivedType(tag: DW_TAG_member, name: "encrypt", scope: !35, file: !13, line: 34, baseType: !69, size: 64, offset: 320)
!69 = !DIDerivedType(tag: DW_TAG_pointer_type, baseType: !70, size: 64)
!70 = !DISubroutineType(types: !71)
!71 = !{!19, !41, !56, !72, !63, !72}
!72 = !DIDerivedType(tag: DW_TAG_pointer_type, baseType: !58, size: 64)
!73 = !DIDerivedType(tag: DW_TAG_member, name: "decrypt", scope: !35, file: !13, line: 35, baseType: !69, size: 64, offset: 384)
!74 = !DIDerivedType(tag: DW_TAG_member, name: "ivsize", scope: !35, file: !13, line: 36, baseType: !75, size: 64, offset: 448)
!75 = !DIDerivedType(tag: DW_TAG_pointer_type, baseType: !76, size: 64)
!76 = !DISubroutineType(types: !77)
!77 = !{!63, !41}
!78 = !DIDerivedType(tag: DW_TAG_member, name: "statesize", scope: !35, file: !13, line: 37, baseType: !75, size: 64, offset: 512)
!79 = !DIDerivedType(tag: DW_TAG_member, name: "get_flags", scope: !35, file: !13, line: 38, baseType: !80, size: 64, offset: 576)
!80 = !DIDerivedType(tag: DW_TAG_pointer_type, baseType: !81, size: 64)
!81 = !DISubroutineType(types: !82)
!82 = !{!83, !41}
!83 = !DIDerivedType(tag: DW_TAG_typedef, name: "uint32_t", file: !59, line: 26, baseType: !84)
!84 = !DIDerivedType(tag: DW_TAG_typedef, name: "__uint32_t", file: !61, line: 42, baseType: !63)
!85 = !DIDerivedType(tag: DW_TAG_member, name: "owner", scope: !35, file: !13, line: 39, baseType: !41, size: 64, offset: 640)
!86 = !DIDerivedType(tag: DW_TAG_member, name: "name", scope: !35, file: !13, line: 40, baseType: !87, size: 112, offset: 704)
!87 = !DICompositeType(tag: DW_TAG_array_type, baseType: !8, size: 112, elements: !88)
!88 = !{!89}
!89 = !DISubrange(count: 14)
!90 = !DIDerivedType(tag: DW_TAG_member, name: "tfm", scope: !30, file: !13, line: 44, baseType: !41, size: 64, offset: 64)
!91 = !DIDerivedType(tag: DW_TAG_member, name: "siv_len", scope: !30, file: !13, line: 45, baseType: !83, size: 32, offset: 128)
!92 = !DIDerivedType(tag: DW_TAG_member, name: "rcu", scope: !30, file: !13, line: 46, baseType: !93, size: 128, offset: 192)
!93 = distinct !DICompositeType(tag: DW_TAG_structure_type, name: "callback_head", file: !13, line: 16, size: 128, elements: !94)
!94 = !{!95, !97}
!95 = !DIDerivedType(tag: DW_TAG_member, name: "next", scope: !93, file: !13, line: 17, baseType: !96, size: 64)
!96 = !DIDerivedType(tag: DW_TAG_pointer_type, baseType: !93, size: 64)
!97 = !DIDerivedType(tag: DW_TAG_member, name: "func", scope: !93, file: !13, line: 18, baseType: !98, size: 64, offset: 64)
!98 = !DIDerivedType(tag: DW_TAG_pointer_type, baseType: !99, size: 64)
!99 = !DISubroutineType(types: !100)
!100 = !{null, !96}
!101 = !DIDerivedType(tag: DW_TAG_member, name: "usage", scope: !30, file: !13, line: 47, baseType: !102, size: 32, offset: 320)
!102 = !DIDerivedType(tag: DW_TAG_typedef, name: "refcount_t", file: !13, line: 27, baseType: !103)
!103 = distinct !DICompositeType(tag: DW_TAG_structure_type, name: "refcount_struct", file: !13, line: 24, size: 32, elements: !104)
!104 = !{!105}
!105 = !DIDerivedType(tag: DW_TAG_member, name: "refs", scope: !103, file: !13, line: 25, baseType: !106, size: 32)
!106 = !DIDerivedType(tag: DW_TAG_typedef, name: "atomic_t", file: !13, line: 22, baseType: !107)
!107 = distinct !DICompositeType(tag: DW_TAG_structure_type, file: !13, line: 20, size: 32, elements: !108)
!108 = !{!109}
!109 = !DIDerivedType(tag: DW_TAG_member, name: "counter", scope: !107, file: !13, line: 21, baseType: !19, size: 32)
!110 = !{!111}
!111 = !{!"btf_type_tag", !"kptr"}
!112 = !DIDerivedType(tag: DW_TAG_member, name: "max_entries", scope: !14, file: !13, line: 69, baseType: !113, size: 64, offset: 192)
!113 = !DIDerivedType(tag: DW_TAG_pointer_type, baseType: !114, size: 64)
!114 = !DICompositeType(tag: DW_TAG_array_type, baseType: !19, size: 32, elements: !115)
!115 = !{!116}
!116 = !DISubrange(count: 1)
!117 = !DIGlobalVariableExpression(var: !118, expr: !DIExpression())
!118 = distinct !DIGlobalVariable(name: "status", scope: !2, file: !3, line: 31, type: !19, isLocal: false, isDefinition: true)
!119 = !DIGlobalVariableExpression(var: !120, expr: !DIExpression())
!120 = distinct !DIGlobalVariable(name: "dummy", scope: !2, file: !3, line: 33, type: !121, isLocal: false, isDefinition: true)
!121 = !DIDerivedType(tag: DW_TAG_volatile_type, baseType: !26)
!122 = !DIGlobalVariableExpression(var: !123, expr: !DIExpression())
!123 = distinct !DIGlobalVariable(name: "bpf_map_update_elem", scope: !2, file: !124, line: 78, type: !125, isLocal: true, isDefinition: true)
!124 = !DIFile(filename: "/usr/include/bpf/bpf_helper_defs.h", directory: "", checksumkind: CSK_MD5, checksum: "ba0039f6acc4710a5f4349e628ddfb60")
!125 = !DIDerivedType(tag: DW_TAG_pointer_type, baseType: !126, size: 64)
!126 = !DISubroutineType(types: !127)
!127 = !{!128, !41, !129, !129, !131}
!128 = !DIBasicType(name: "long", size: 64, encoding: DW_ATE_signed)
!129 = !DIDerivedType(tag: DW_TAG_pointer_type, baseType: !130, size: 64)
!130 = !DIDerivedType(tag: DW_TAG_const_type, baseType: null)
!131 = !DIDerivedType(tag: DW_TAG_typedef, name: "__u64", file: !132, line: 31, baseType: !133)
!132 = !DIFile(filename: "/usr/include/asm-generic/int-ll64.h", directory: "", checksumkind: CSK_MD5, checksum: "b810f270733e106319b67ef512c6246e")
!133 = !DIBasicType(name: "unsigned long long", size: 64, encoding: DW_ATE_unsigned)
!134 = !DIGlobalVariableExpression(var: !135, expr: !DIExpression())
!135 = distinct !DIGlobalVariable(name: "bpf_map_lookup_elem", scope: !2, file: !124, line: 56, type: !136, isLocal: true, isDefinition: true)
!136 = !DIDerivedType(tag: DW_TAG_pointer_type, baseType: !137, size: 64)
!137 = !DISubroutineType(types: !138)
!138 = !{!41, !41, !129}
!139 = !DIGlobalVariableExpression(var: !140, expr: !DIExpression())
!140 = distinct !DIGlobalVariable(name: "bpf_kptr_xchg", scope: !2, file: !124, line: 4399, type: !141, isLocal: true, isDefinition: true)
!141 = !DIDerivedType(tag: DW_TAG_pointer_type, baseType: !142, size: 64)
!142 = !DISubroutineType(types: !143)
!143 = !{!41, !41, !41}
!144 = !DIDerivedType(tag: DW_TAG_volatile_type, baseType: !145)
!145 = !DIDerivedType(tag: DW_TAG_typedef, name: "bpf_crypto_ctx_t", file: !13, line: 48, baseType: !30)
!146 = !{i32 7, !"Dwarf Version", i32 5}
!147 = !{i32 2, !"Debug Info Version", i32 3}
!148 = !{i32 1, !"wchar_size", i32 4}
!149 = !{i32 7, !"PIC Level", i32 2}
!150 = !{i32 7, !"PIE Level", i32 2}
!151 = !{i32 7, !"uwtable", i32 1}
!152 = !{!"Debian clang version 14.0.6"}
!153 = distinct !DISubprogram(name: "crypto_init", scope: !3, file: !3, line: 35, type: !154, scopeLine: 35, flags: DIFlagPrototyped | DIFlagAllCallsDescribed, spFlags: DISPFlagDefinition | DISPFlagOptimized, unit: !2, retainedNodes: !156)
!154 = !DISubroutineType(types: !155)
!155 = !{!19, !41}
!156 = !{!157, !158, !160, !176}
!157 = !DILocalVariable(name: "args", arg: 1, scope: !153, file: !3, line: 35, type: !41)
!158 = !DILocalVariable(name: "cctx", scope: !153, file: !3, line: 38, type: !159)
!159 = !DIDerivedType(tag: DW_TAG_pointer_type, baseType: !145, size: 64, annotations: !110)
!160 = !DILocalVariable(name: "params", scope: !153, file: !3, line: 39, type: !161)
!161 = distinct !DICompositeType(tag: DW_TAG_structure_type, name: "bpf_crypto_params", file: !13, line: 8, size: 3264, elements: !162)
!162 = !{!163, !164, !166, !170, !174, !175}
!163 = !DIDerivedType(tag: DW_TAG_member, name: "type", scope: !161, file: !13, line: 9, baseType: !87, size: 112)
!164 = !DIDerivedType(tag: DW_TAG_member, name: "reserved", scope: !161, file: !13, line: 10, baseType: !165, size: 16, offset: 112)
!165 = !DICompositeType(tag: DW_TAG_array_type, baseType: !58, size: 16, elements: !20)
!166 = !DIDerivedType(tag: DW_TAG_member, name: "algo", scope: !161, file: !13, line: 11, baseType: !167, size: 1024, offset: 128)
!167 = !DICompositeType(tag: DW_TAG_array_type, baseType: !8, size: 1024, elements: !168)
!168 = !{!169}
!169 = !DISubrange(count: 128)
!170 = !DIDerivedType(tag: DW_TAG_member, name: "key", scope: !161, file: !13, line: 12, baseType: !171, size: 2048, offset: 1152)
!171 = !DICompositeType(tag: DW_TAG_array_type, baseType: !58, size: 2048, elements: !172)
!172 = !{!173}
!173 = !DISubrange(count: 256)
!174 = !DIDerivedType(tag: DW_TAG_member, name: "key_len", scope: !161, file: !13, line: 13, baseType: !83, size: 32, offset: 3200)
!175 = !DIDerivedType(tag: DW_TAG_member, name: "authsize", scope: !161, file: !13, line: 14, baseType: !83, size: 32, offset: 3232)
!176 = !DILocalVariable(name: "err", scope: !153, file: !3, line: 45, type: !19)
!177 = !DILocation(line: 0, scope: !153)
!178 = !DILocation(line: 37, column: 8, scope: !153)
!179 = !{i64 0, i64 8, !180, i64 8, i64 8, !180, i64 16, i64 4, !184, i64 24, i64 8, !180, i64 32, i64 8, !180, i64 40, i64 4, !184}
!180 = !{!181, !181, i64 0}
!181 = !{!"any pointer", !182, i64 0}
!182 = !{!"omnipotent char", !183, i64 0}
!183 = !{!"Simple C/C++ TBAA"}
!184 = !{!185, !185, i64 0}
!185 = !{!"int", !182, i64 0}
!186 = !DILocation(line: 39, column: 2, scope: !153)
!187 = !DILocation(line: 39, column: 27, scope: !153)
!188 = !DILocation(line: 45, column: 2, scope: !153)
!189 = !DILocation(line: 45, column: 6, scope: !153)
!190 = !DILocation(line: 47, column: 9, scope: !153)
!191 = !DILocation(line: 50, column: 9, scope: !153)
!192 = !DILocation(line: 52, column: 7, scope: !193)
!193 = distinct !DILexicalBlock(scope: !153, file: !3, line: 52, column: 6)
!194 = !DILocation(line: 52, column: 6, scope: !153)
!195 = !DILocation(line: 53, column: 12, scope: !196)
!196 = distinct !DILexicalBlock(scope: !193, file: !3, line: 52, column: 13)
!197 = !DILocation(line: 53, column: 10, scope: !196)
!198 = !DILocation(line: 54, column: 3, scope: !196)
!199 = !DILocalVariable(name: "ctx", arg: 1, scope: !200, file: !3, line: 3, type: !203)
!200 = distinct !DISubprogram(name: "crypto_ctx_insert", scope: !3, file: !3, line: 3, type: !201, scopeLine: 4, flags: DIFlagPrototyped | DIFlagAllCallsDescribed, spFlags: DISPFlagDefinition | DISPFlagOptimized, unit: !2, retainedNodes: !204)
!201 = !DISubroutineType(types: !202)
!202 = !{!19, !203}
!203 = !DIDerivedType(tag: DW_TAG_pointer_type, baseType: !145, size: 64)
!204 = !{!199, !205, !206, !207, !208, !209}
!205 = !DILocalVariable(name: "local", scope: !200, file: !3, line: 5, type: !26)
!206 = !DILocalVariable(name: "v", scope: !200, file: !3, line: 5, type: !25)
!207 = !DILocalVariable(name: "old", scope: !200, file: !3, line: 6, type: !203)
!208 = !DILocalVariable(name: "key", scope: !200, file: !3, line: 7, type: !83)
!209 = !DILocalVariable(name: "err", scope: !200, file: !3, line: 8, type: !19)
!210 = !DILocation(line: 0, scope: !200, inlinedAt: !211)
!211 = distinct !DILocation(line: 57, column: 8, scope: !153)
!212 = !DILocation(line: 5, column: 2, scope: !200, inlinedAt: !211)
!213 = !DILocation(line: 5, column: 28, scope: !200, inlinedAt: !211)
!214 = !DILocation(line: 7, column: 2, scope: !200, inlinedAt: !211)
!215 = !DILocation(line: 7, column: 11, scope: !200, inlinedAt: !211)
!216 = !DILocation(line: 10, column: 8, scope: !200, inlinedAt: !211)
!217 = !DILocation(line: 10, column: 12, scope: !200, inlinedAt: !211)
!218 = !{!219, !181, i64 0}
!219 = !{!"__crypto_ctx_value", !181, i64 0}
!220 = !DILocation(line: 11, column: 8, scope: !200, inlinedAt: !211)
!221 = !DILocation(line: 12, column: 6, scope: !222, inlinedAt: !211)
!222 = distinct !DILexicalBlock(scope: !200, file: !3, line: 12, column: 6)
!223 = !DILocation(line: 12, column: 6, scope: !200, inlinedAt: !211)
!224 = !DILocation(line: 17, column: 6, scope: !200, inlinedAt: !211)
!225 = !DILocation(line: 18, column: 7, scope: !226, inlinedAt: !211)
!226 = distinct !DILexicalBlock(scope: !200, file: !3, line: 18, column: 6)
!227 = !DILocation(line: 18, column: 6, scope: !200, inlinedAt: !211)
!228 = !DILocation(line: 19, column: 3, scope: !229, inlinedAt: !211)
!229 = distinct !DILexicalBlock(scope: !226, file: !3, line: 18, column: 10)
!230 = !DILocation(line: 29, column: 1, scope: !200, inlinedAt: !211)
!231 = !DILocation(line: 58, column: 10, scope: !232)
!232 = distinct !DILexicalBlock(scope: !153, file: !3, line: 58, column: 6)
!233 = !DILocation(line: 22, column: 31, scope: !200, inlinedAt: !211)
!234 = !DILocation(line: 22, column: 8, scope: !200, inlinedAt: !211)
!235 = !DILocation(line: 23, column: 6, scope: !236, inlinedAt: !211)
!236 = distinct !DILexicalBlock(scope: !200, file: !3, line: 23, column: 6)
!237 = !DILocation(line: 23, column: 6, scope: !200, inlinedAt: !211)
!238 = !DILocation(line: 24, column: 3, scope: !239, inlinedAt: !211)
!239 = distinct !DILexicalBlock(scope: !236, file: !3, line: 23, column: 11)
!240 = !DILocation(line: 25, column: 3, scope: !239, inlinedAt: !211)
!241 = !DILocation(line: 13, column: 3, scope: !242, inlinedAt: !211)
!242 = distinct !DILexicalBlock(scope: !222, file: !3, line: 12, column: 11)
!243 = !DILocation(line: 59, column: 10, scope: !232)
!244 = !DILocation(line: 59, column: 3, scope: !232)
!245 = !DILocation(line: 63, column: 1, scope: !153)
!246 = !DISubprogram(name: "bpf_crypto_ctx_create", scope: !13, file: !13, line: 52, type: !247, flags: DIFlagPrototyped, spFlags: DISPFlagOptimized, retainedNodes: !251)
!247 = !DISubroutineType(types: !248)
!248 = !{!203, !249, !83, !23}
!249 = !DIDerivedType(tag: DW_TAG_pointer_type, baseType: !250, size: 64)
!250 = !DIDerivedType(tag: DW_TAG_const_type, baseType: !161)
!251 = !{}
!252 = !DISubprogram(name: "bpf_crypto_ctx_release", scope: !13, file: !13, line: 58, type: !253, flags: DIFlagPrototyped, spFlags: DISPFlagOptimized, retainedNodes: !251)
!253 = !DISubroutineType(types: !254)
!254 = !{null, !203}
