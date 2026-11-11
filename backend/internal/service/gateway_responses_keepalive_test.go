//go:build unit
package service
import (
 "context"
 "io"
 "os"
 "net/http"
 "net/http/httptest"
 "strings"
 "testing"
 "time"
 "github.com/Wei-Shaw/sub2api/internal/config"
 "github.com/Wei-Shaw/sub2api/internal/pkg/apicompat"
 "github.com/gin-gonic/gin"
 "github.com/stretchr/testify/require"
)
func keepaliveContext()(*gin.Context,*httptest.ResponseRecorder){
 gin.SetMode(gin.TestMode);r:=httptest.NewRecorder();c,_:=gin.CreateTestContext(r);c.Request=httptest.NewRequest("POST","/v1/responses",nil);return c,r
}
func TestResponsesKeepalive_UpstreamHeadersAndError(t *testing.T){
 c,r:=keepaliveContext();s:=&GatewayService{cfg:&config.Config{Gateway:config.GatewayConfig{StreamKeepaliveInterval:1}}}
 req:=httptest.NewRequest("POST","http://upstream/v1/messages",nil)
 resp,err,cancel:=s.doResponsesUpstreamWithKeepalive(c,true,req,func(*http.Request)(*http.Response,error){time.Sleep(1200*time.Millisecond);return &http.Response{StatusCode:403,Body:io.NopCloser(strings.NewReader("denied"))},nil});defer cancel()
 require.NoError(t,err);require.Equal(t,403,resp.StatusCode);resp.Body.Close()
 writeResponsesError(c,403,"permission_error","denied")
 require.Equal(t,200,r.Code);require.Contains(t,r.Body.String(),"event: ping\n");require.Contains(t,r.Body.String(),"event: error\n");require.NotContains(t,r.Body.String(),"response.completed")
}
func TestResponsesKeepalive_NonStreamingPreservesHTTPError(t *testing.T){
 c,r:=keepaliveContext();s:=&GatewayService{}
 resp,err,cancel:=s.doResponsesUpstreamWithKeepalive(c,false,c.Request,func(*http.Request)(*http.Response,error){return &http.Response{StatusCode:403},nil});defer cancel()
 require.NoError(t,err);require.Equal(t,403,resp.StatusCode);require.Empty(t,r.Body.String());writeResponsesError(c,403,"permission_error","denied");require.Equal(t,403,r.Code)
}
func TestResponsesKeepalive_CancelsHeaderWorker(t *testing.T){
 c,_:=keepaliveContext();ctx,cancelClient:=context.WithCancel(c.Request.Context());c.Request=c.Request.WithContext(ctx);s:=&GatewayService{};exit:=make(chan struct{})
 go func(){time.Sleep(20*time.Millisecond);cancelClient()}()
 _,err,cancel:=s.doResponsesUpstreamWithKeepalive(c,true,c.Request,func(r *http.Request)(*http.Response,error){<-r.Context().Done();close(exit);return nil,r.Context().Err()});defer cancel()
 require.ErrorIs(t,err,context.Canceled);select{case <-exit:case <-time.After(time.Second):t.Fatal("upstream worker leaked")}
}
func TestResponsesKeepalive_SilentBodyAndUsage(t *testing.T){
 c,r:=keepaliveContext();s:=&GatewayService{cfg:&config.Config{Gateway:config.GatewayConfig{StreamKeepaliveInterval:1,StreamDataIntervalTimeout:180}}};reader,writer:=io.Pipe()
 go func(){defer writer.Close();time.Sleep(1200*time.Millisecond)
 io.WriteString(writer,"event: message_start\ndata: {\"type\":\"message_start\",\"message\":{\"id\":\"msg_test\",\"model\":\"claude-opus-5-5\",\"usage\":{\"input_tokens\":12}}}\n\n")
 if os.Getenv("KEEPALIVE_LONG_TEST")=="1" {time.Sleep(130*time.Second)} else {time.Sleep(1200*time.Millisecond)}
 io.WriteString(writer,"event: message_delta\ndata: {\"type\":\"message_delta\",\"delta\":{\"stop_reason\":\"end_turn\"},\"usage\":{\"output_tokens\":7}}\n\nevent: message_stop\ndata: {\"type\":\"message_stop\"}\n\n") }()
 result,err:=s.handleResponsesStreamingResponse(&http.Response{Header:http.Header{},Body:reader},c,"claude-opus-5-5","claude-opus-5-5",nil,time.Now(),apicompat.ResponsesClientToolMapping{})
 require.NoError(t,err);require.GreaterOrEqual(t,strings.Count(r.Body.String(),"event: ping\n"),2);require.Contains(t,r.Body.String(),"response.completed");require.Equal(t,12,result.Usage.InputTokens);require.Equal(t,7,result.Usage.OutputTokens)
}
func TestResponsesKeepalive_TruncatedBodyIsError(t *testing.T){
 c,r:=keepaliveContext();_,err:=(&GatewayService{}).handleResponsesStreamingResponse(&http.Response{Header:http.Header{},Body:io.NopCloser(strings.NewReader("event: message_start\ndata: {\"type\":\"message_start\",\"message\":{\"id\":\"msg_cut\",\"usage\":{\"input_tokens\":1}}}\n\n"))},c,"claude-opus-5-5","claude-opus-5-5",nil,time.Now(),apicompat.ResponsesClientToolMapping{})
 require.Error(t,err);require.Contains(t,r.Body.String(),"event: error");require.NotContains(t,r.Body.String(),"response.completed")
}
