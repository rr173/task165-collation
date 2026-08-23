package service
import("context";"testing")
func TestBug07_SnapshotViewRetainsHashValue(t *testing.T){svc,_:=newTestService(t);c:=context.Background();p,_:=svc.CreateProject(c,"哈希视图","");b,_:=svc.CreateWitness(c,p.ID,"B","底本","",true);if _,e:=svc.ImportPassages(c,b.ID,"哈希正文。");e!=nil{t.Fatal(e)};s,e:=svc.BuildSnapshot(c,p.ID);if e!=nil{t.Fatal(e)};v,e:=svc.GetSnapshot(c,s.ID);if e!=nil{t.Fatal(e)};for _,h:=range v.PassageHashes{if h==""{t.Fatal("snapshot hash value missing")};return};t.Fatal("snapshot hash missing")}
